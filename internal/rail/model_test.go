package rail

import (
	"errors"
	"testing"
)

func TestCreateTransferRequestValidateDefaultsBRL(t *testing.T) {
	req := CreateTransferRequest{
		TenantID:        "tenant_a",
		AccountID:       "acct_1",
		IdempotencyKey:  "idem",
		AmountCents:     100,
		ReceiverKey:     "receiver@example.com",
		ReceiverKeyType: KeyEmail,
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected valid request with default currency, got %v", err)
	}
}

func TestCreateTransferRequestValidateRejectsInvalidInput(t *testing.T) {
	req := CreateTransferRequest{Currency: "USD"}
	err := req.Validate()
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestTransferStatusTerminal(t *testing.T) {
	if !StatusSettled.Terminal() || !StatusBlocked.Terminal() || !StatusRejected.Terminal() {
		t.Fatal("expected settled, blocked, and rejected to be terminal")
	}
	if StatusApproved.Terminal() || StatusReview.Terminal() {
		t.Fatal("approved and review are not terminal")
	}
}
