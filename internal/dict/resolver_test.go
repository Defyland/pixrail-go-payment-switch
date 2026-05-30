package dict

import (
	"context"
	"errors"
	"testing"

	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
)

func TestStaticResolverReturnsDeterministicEntry(t *testing.T) {
	resolver := StaticResolver{}

	entry, err := resolver.Resolve(context.Background(), "tenant_a", "receiver@example.com", rail.KeyEmail)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if entry.ReceiverID == "" || entry.BankISPB == "" || entry.AccountHash == "" {
		t.Fatalf("entry missing resolved identifiers: %+v", entry)
	}
}

func TestStaticResolverSimulatesTimeout(t *testing.T) {
	resolver := StaticResolver{}

	_, err := resolver.Resolve(context.Background(), "tenant_a", "timeout@example.com", rail.KeyEmail)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout, got %v", err)
	}
}
