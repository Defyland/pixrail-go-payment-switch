package app

import (
	"context"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/config"
	"github.com/Defyland/pixrail-go-payment-switch/internal/dict"
	"github.com/Defyland/pixrail-go-payment-switch/internal/fraud"
	"github.com/Defyland/pixrail-go-payment-switch/internal/postgres"
	"github.com/Defyland/pixrail-go-payment-switch/internal/ratelimit"
	"github.com/Defyland/pixrail-go-payment-switch/internal/spi"
	memorystore "github.com/Defyland/pixrail-go-payment-switch/internal/store"
	"github.com/Defyland/pixrail-go-payment-switch/internal/switcher"
)

func BuildStore(ctx context.Context, cfg config.Config) (switcher.Store, func(), error) {
	switch cfg.StoreDriver {
	case "memory":
		return memorystore.NewMemoryStore(), func() {}, nil
	case "postgres":
		store, err := postgres.New(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, nil, err
		}
		return store, store.Close, nil
	default:
		return nil, nil, nil
	}
}

func NewService(cfg config.Config, store switcher.Store) *switcher.Service {
	return switcher.NewService(
		store,
		dict.StaticResolver{TimeoutSignal: cfg.DictTimeout},
		fraud.RulesEngine{},
		spi.Simulator{},
		ratelimit.New(ratelimit.BucketConfig{Capacity: cfg.TenantBucketSize, RefillTokens: cfg.TenantBucketSize, RefillEvery: time.Minute}),
		ratelimit.New(ratelimit.BucketConfig{Capacity: cfg.DictBucketSize, RefillTokens: cfg.DictBucketSize, RefillEvery: time.Minute}),
	)
}
