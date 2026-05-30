package dict

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
)

type Resolver interface {
	Resolve(ctx context.Context, tenantID string, key string, keyType rail.DictKeyType) (rail.DictEntry, error)
}

type StaticResolver struct {
	Latency       time.Duration
	TimeoutSignal time.Duration
}

func (r StaticResolver) Resolve(ctx context.Context, tenantID string, key string, keyType rail.DictKeyType) (rail.DictEntry, error) {
	if r.Latency > 0 {
		timer := time.NewTimer(r.Latency)
		select {
		case <-ctx.Done():
			timer.Stop()
			return rail.DictEntry{}, ctx.Err()
		case <-timer.C:
		}
	}

	normalized := strings.ToLower(strings.TrimSpace(key))
	if strings.Contains(normalized, "missing") {
		return rail.DictEntry{}, fmt.Errorf("%w: dict key not found", rail.ErrDependencyFailed)
	}
	if strings.Contains(normalized, "timeout") {
		timeout := r.TimeoutSignal
		if timeout <= 0 {
			timeout = 20 * time.Millisecond
		}
		timer := time.NewTimer(timeout)
		select {
		case <-ctx.Done():
			timer.Stop()
			return rail.DictEntry{}, ctx.Err()
		case <-timer.C:
			return rail.DictEntry{}, context.DeadlineExceeded
		}
	}

	hash := sha256.Sum256([]byte(tenantID + ":" + normalized))
	risk := int(hash[0] % 35)
	name := "Recebedor PixRail"
	if strings.Contains(normalized, "mule") || strings.Contains(normalized, "risk") {
		risk = 92
		name = "Conta de Alto Risco"
	}

	return rail.DictEntry{
		Key:         key,
		KeyType:     keyType,
		ReceiverID:  "dict_" + hex.EncodeToString(hash[:6]),
		Name:        name,
		BankISPB:    fmt.Sprintf("%08d", 10000000+int(hash[1])*1000+int(hash[2])),
		AccountHash: hex.EncodeToString(hash[:12]),
		RiskSignal:  risk,
		ResolvedAt:  time.Now().UTC(),
	}, nil
}
