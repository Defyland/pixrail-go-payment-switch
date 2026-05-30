package fraud

import (
	"context"

	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
)

type Engine interface {
	Score(ctx context.Context, transfer rail.CreateTransferRequest, dict rail.DictEntry) (rail.FraudDecision, error)
}

type RulesEngine struct{}

func (RulesEngine) Score(_ context.Context, transfer rail.CreateTransferRequest, dict rail.DictEntry) (rail.FraudDecision, error) {
	score := dict.RiskSignal
	rules := make([]string, 0, 4)

	if transfer.AmountCents >= 1_000_000 {
		score += 40
		rules = append(rules, "amount_high")
	}
	if transfer.AmountCents >= 500_000 {
		score += 15
		rules = append(rules, "amount_review")
	}
	if dict.RiskSignal >= 80 {
		score += 35
		rules = append(rules, "dict_high_risk")
	}
	if transfer.AccountID == dict.AccountHash {
		score += 25
		rules = append(rules, "self_transfer_hash_match")
	}

	decision := rail.FraudDecision{
		Score:  score,
		Status: rail.StatusApproved,
		Rules:  rules,
		Reason: "risk within payment-rail policy",
	}
	switch {
	case score >= 90:
		decision.Status = rail.StatusBlocked
		decision.Reason = "blocked by fraud policy"
	case score >= 65:
		decision.Status = rail.StatusReview
		decision.Reason = "manual review required"
	}
	return decision, nil
}
