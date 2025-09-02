package tool

import "github.com/ethereum/go-ethereum/metrics"

var (
	ingressRegistrationErrorName = "eth/protocols/tool/ingress/registration/error"
	egressRegistrationErrorName  = "eth/protocols/tool/egress/registration/error"

	IngressRegistrationErrorMeter = metrics.NewRegisteredMeter(ingressRegistrationErrorName, nil)
	EgressRegistrationErrorMeter  = metrics.NewRegisteredMeter(egressRegistrationErrorName, nil)
)
