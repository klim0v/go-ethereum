package relay

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	v1 "github.com/attestantio/go-builder-client/api/v1"
	beacon "github.com/attestantio/go-eth2-client/api"
	"github.com/attestantio/go-eth2-client/api/v1/electra"
	"github.com/attestantio/go-eth2-client/spec"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/klauspost/compress/gzhttp"
)

type Server struct {
	srv     *http.Server
	relay   *Relay
	builder envelopeBuilder
}

func NewServer(relay *Relay) *Server {
	return &Server{relay: relay}
}

func (s *Server) Start() error {
	cfg := s.relay.cfg
	mux := http.NewServeMux()
	mux.HandleFunc("GET /eth/v1/builder/status", recoveryMiddleware(loggingMiddleware(s.handleStatus)))
	mux.HandleFunc("POST /eth/v1/builder/validators", recoveryMiddleware(loggingMiddleware(s.handleRegisterValidators)))
	mux.HandleFunc("GET /eth/v1/builder/header/{slot}/{parentHash}/{pubKey}", recoveryMiddleware(loggingMiddleware(s.handleGetHeader)))
	mux.HandleFunc("POST /eth/v1/builder/blinded_blocks", recoveryMiddleware(loggingMiddleware(s.handleGetPayload)))
	s.srv = &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: gzhttp.GzipHandler(mux),

		ReadTimeout:       12 * time.Second,
		ReadHeaderTimeout: 6 * time.Second,
		WriteTimeout:      12 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    http.DefaultMaxHeaderBytes,

		DisableGeneralOptionsHandler: false,
	}

	go func() {
		log.Info("Starting relay server", "addr", cfg.ListenAddr)
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Crit("Failed to start relay server", "error", err)
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := s.srv.Shutdown(ctx)
	if err != nil {
		return err
	}

	return nil
}

const (
	headerEthConsensusVersion = "Eth-Consensus-Version"
	headerContentType         = "Content-Type"
	headerAccept              = "Accept"
	applicationJSON           = "application/json"
	applicationOctetStream    = "application/octet-stream"
)

func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		format, _ := getFormats(r.Header)
		consensusVersion := r.Header.Get(headerEthConsensusVersion)
		clientIP := getClientIP(r)

		log.Debug("Request received",
			"ip", clientIP,
			"format", format,
			"method", r.Method,
			"path", r.URL.Path,
			"consensusVersion", consensusVersion,
			"contentLength", r.ContentLength,
		)

		next(w, r)

		log.Debug("Request completed",
			"ip", clientIP,
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start),
		)
	}
}

func recoveryMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("Panic recovered", "error", err, "stack", string(debug.Stack()))
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next(w, r)
	}
}

func extractMediaType(value string) string {
	if strings.Contains(value, applicationOctetStream) {
		return applicationOctetStream
	}
	return applicationJSON
}

func getFormats(header http.Header) (string, string) {
	rawContentType := header.Get(headerContentType)
	rawAccept := header.Get(headerAccept)

	if rawContentType == "" && rawAccept == "" {
		return applicationJSON, applicationJSON
	}

	if rawContentType == "" {
		rawContentType = rawAccept
	}
	if rawAccept == "" {
		rawAccept = rawContentType
	}

	return extractMediaType(rawContentType), extractMediaType(rawAccept)
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleRegisterValidators(w http.ResponseWriter, r *http.Request) {
	reqFormat, _ := getFormats(r.Header)

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024*1024))
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var registrations v1.SignedValidatorRegistrations
	switch reqFormat {
	case applicationOctetStream:
		if err := registrations.UnmarshalSSZ(body); err != nil {
			http.Error(w, "failed to decode SSZ payload", http.StatusBadRequest)
			return
		}
	default:
		if err := registrations.UnmarshalJSON(body); err != nil {
			http.Error(w, "failed to decode JSON payload", http.StatusBadRequest)
			return
		}
	}

	if len(registrations.Registrations) > validatorRegistrationCheckCount {
		http.Error(w, "too many registrations", http.StatusBadRequest)
		return
	}

	if err := s.relay.RegisterValidators(r.Context(), registrations.Registrations); err != nil {
		http.Error(w, "failed to register validator", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGetPayload(w http.ResponseWriter, r *http.Request) {
	reqFormat, respFormat := getFormats(r.Header)

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 15*1024*1024))
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var signedBlindedBlock electra.SignedBlindedBeaconBlock
	switch reqFormat {
	case applicationOctetStream:
		if err := signedBlindedBlock.UnmarshalSSZ(body); err != nil {
			http.Error(w, "failed to decode SSZ payload", http.StatusBadRequest)
			return
		}
	default:
		if err := signedBlindedBlock.UnmarshalJSON(body); err != nil {
			http.Error(w, "failed to decode JSON payload", http.StatusBadRequest)
			return
		}
	}

	payload, err := s.relay.GetPayload(r.Context(), &beacon.VersionedSignedBlindedBeaconBlock{
		Version: spec.DataVersionElectra,
		Electra: &signedBlindedBlock,
	})
	if err != nil {
		http.Error(w, "failed to get payload", http.StatusBadRequest)
		return
	}

	var response []byte
	headers := map[string]string{headerContentType: respFormat, headerEthConsensusVersion: payload.Version.String()}
	if respFormat == applicationOctetStream {
		response, err = payload.Electra.MarshalSSZ()
	} else {
		response, err = payload.MarshalJSON()
	}
	if err != nil {
		log.Error("Failed to encode payload", "error", err)
		http.Error(w, "failed to encode payload", http.StatusInternalServerError)
		return
	}
	for k, v := range headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(http.StatusOK)
	if _, err = w.Write(response); err != nil {
		log.Error("Failed to write getPayload response", "error", err)
		return
	}
}

func (s *Server) handleGetHeader(w http.ResponseWriter, r *http.Request) {
	_, respFormat := getFormats(r.Header)

	slot, err := strconv.ParseUint(r.PathValue("slot"), 10, 64)
	if err != nil {
		http.Error(w, "invalid slot", http.StatusBadRequest)
		return
	}

	parentHash, err := hexutil.Decode(r.PathValue("parentHash"))
	if err != nil || len(parentHash) != 32 {
		http.Error(w, "invalid parent hash", http.StatusBadRequest)
		return
	}

	pubKey, err := hexutil.Decode(r.PathValue("pubKey"))
	if err != nil || len(pubKey) != 48 {
		http.Error(w, "invalid pubKey", http.StatusBadRequest)
		return
	}

	header, err := s.relay.GetHeader(r.Context(), phase0.Slot(slot), phase0.Hash32(parentHash), phase0.BLSPubKey(pubKey))
	if err != nil {
		http.Error(w, "failed to get header", http.StatusNotFound)
		return
	}

	var response []byte
	headers := map[string]string{headerContentType: respFormat, headerEthConsensusVersion: header.Version.String()}
	if respFormat == applicationOctetStream {
		response, err = header.Electra.MarshalSSZ()
	} else {
		response, err = header.MarshalJSON()
	}
	if err != nil {
		log.Error("Failed to encode header", "error", err)
		http.Error(w, "failed to encode header", http.StatusInternalServerError)
		return
	}
	for k, v := range headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(http.StatusOK)
	if _, err = w.Write(response); err != nil {
		log.Error("Failed to write getHeader response", "error", err)
		return
	}
}

// Extract real client IP from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (most common)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (client IP)
		ips := strings.Split(xff, ",")
		clientIP := strings.TrimSpace(ips[0])
		if net.ParseIP(clientIP) != nil {
			return clientIP
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	// Check CF-Connecting-IP (Cloudflare)
	if cfip := r.Header.Get("CF-Connecting-IP"); cfip != "" {
		if net.ParseIP(cfip) != nil {
			return cfip
		}
	}

	// Fallback to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
