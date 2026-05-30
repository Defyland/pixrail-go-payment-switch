package fraud

import (
	"context"
	"testing"

	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
)

func TestRulesEngineBlocksHighRiskDictEntry(t *testing.T) {
	decision, err := RulesEngine{}.Score(context.Background(), rail.CreateTransferRequest{AmountCents: 120_000}, rail.DictEntry{RiskSignal: 95})
	if err != nil {
		t.Fatalf("score failed: %v", err)
	}
	if decision.Status != rail.StatusBlocked {
		t.Fatalf("expected blocked, got %s", decision.Status)
	}
	if len(decision.Rules) == 0 {
		t.Fatal("expected triggered fraud rules")
	}
}

func TestRulesEngineApprovesLowRiskTransfer(t *testing.T) {
	decision, err := RulesEngine{}.Score(context.Background(), rail.CreateTransferRequest{AmountCents: 5_000}, rail.DictEntry{RiskSignal: 10})
	if err != nil {
		t.Fatalf("score failed: %v", err)
	}
	if decision.Status != rail.StatusApproved {
		t.Fatalf("expected approved, got %s", decision.Status)
	}
}
