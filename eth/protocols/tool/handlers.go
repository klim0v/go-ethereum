package tool

import (
	"time"

	"github.com/ethereum/go-ethereum/p2p/tracker"
)

var requestTracker = tracker.New(ProtocolName, 5*time.Minute)

func handleToolTransactions(backend Backend, msg Decoder, peer *Peer) error {
	if !backend.AcceptData() {
		return nil
	}

	var txs ToolTransactionsPacket
	if err := msg.Decode(&txs); err != nil {
		return err
	}
	for _, tx := range txs.ToolTransactionsResponse {
		peer.markTransaction(tx.Hash())
	}
	requestTracker.Fulfil(peer.id, peer.version, ToolTransactionsMsg, txs.RequestId)

	return backend.Handle(peer, &txs.ToolTransactionsResponse)
}

func handleGetToolTransactions(backend Backend, msg Decoder, peer *Peer) error {
	var query GetToolTransactionsPacket
	if err := msg.Decode(&query); err != nil {
		return err
	}
	txs := backend.GetTxs(query.GetToolTransactionsRequest)
	return peer.ReplyToolTransactions(query.RequestId, txs)
}

func handleCommitments(backend Backend, msg Decoder, peer *Peer) error {
	if !backend.AcceptData() {
		return nil
	}

	var cmts CommitmentsPacket
	if err := msg.Decode(&cmts); err != nil {
		return err
	}
	for _, cmt := range cmts {
		peer.markCommitment(cmt.ID())
		for _, hash := range cmt.Txs {
			peer.markTransaction(hash)
		}
	}

	return backend.Handle(peer, &cmts)
}

func handleSubSlots(backend Backend, msg Decoder, peer *Peer) error {
	if !backend.AcceptData() {
		return nil
	}

	var slots SubSlotPacket
	if err := msg.Decode(&slots); err != nil {
		return err
	}
	for _, subSlot := range slots {
		peer.markSubSlot(subSlot.ID())
		for _, hash := range subSlot.Txs {
			peer.markTransaction(hash)
		}
	}

	return backend.Handle(peer, &slots)
}

func handleValidatorRegs(backend Backend, msg Decoder, peer *Peer) error {
	if !backend.AcceptData() {
		return nil
	}

	var packet ValidatorRegistrations
	if err := msg.Decode(&packet); err != nil {
		return err
	}
	return backend.Handle(peer, &packet)
}
