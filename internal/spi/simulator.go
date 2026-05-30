package spi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
)

type Client interface {
	Submit(ctx context.Context, transfer rail.Transfer) (rail.SPIMessage, error)
}

type Simulator struct{}

func (Simulator) Submit(_ context.Context, transfer rail.Transfer) (rail.SPIMessage, error) {
	sum := sha256.Sum256([]byte(transfer.ID + ":" + transfer.AccountID + ":" + transfer.ReceiverKey))
	return rail.SPIMessage{
		MessageID:   "spi_" + hex.EncodeToString(sum[:8]),
		EndToEndID:  "E" + time.Now().UTC().Format("20060102150405") + hex.EncodeToString(sum[:8]),
		TransferID:  transfer.ID,
		SubmittedAt: time.Now().UTC(),
	}, nil
}
