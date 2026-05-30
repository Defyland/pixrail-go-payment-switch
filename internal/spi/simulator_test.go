package spi

import (
	"context"
	"testing"

	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
)

func TestSimulatorCreatesSPIIdentifiers(t *testing.T) {
	message, err := Simulator{}.Submit(context.Background(), rail.Transfer{ID: "pxt_1", AccountID: "acct_1", ReceiverKey: "receiver@example.com"})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	if message.MessageID == "" || message.EndToEndID == "" || message.TransferID != "pxt_1" {
		t.Fatalf("unexpected message: %+v", message)
	}
}
