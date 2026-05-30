package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type APIKey struct {
	TenantID string
	Secret   string
}

type Config struct {
	Addr                     string
	Environment              string
	APIKeys                  map[string]APIKey
	StoreDriver              string
	DatabaseURL              string
	TenantBucketSize         int
	DictBucketSize           int
	DictTimeout              time.Duration
	ShutdownTimeout          time.Duration
	RequireConfiguredSecrets bool
}

func Load() (Config, error) {
	env := getenv("PIXRAIL_ENV", "development")
	cfg := Config{
		Addr:                     getenv("PIXRAIL_HTTP_ADDR", ":8080"),
		Environment:              env,
		StoreDriver:              getenv("PIXRAIL_STORE_DRIVER", "memory"),
		DatabaseURL:              getenv("PIXRAIL_DATABASE_URL", ""),
		TenantBucketSize:         getenvInt("PIXRAIL_TENANT_BUCKET_SIZE", 120),
		DictBucketSize:           getenvInt("PIXRAIL_DICT_BUCKET_SIZE", 60),
		DictTimeout:              getenvDuration("PIXRAIL_DICT_TIMEOUT", 300*time.Millisecond),
		ShutdownTimeout:          getenvDuration("PIXRAIL_SHUTDOWN_TIMEOUT", 5*time.Second),
		RequireConfiguredSecrets: env == "production",
	}

	keys, err := parseAPIKeys(os.Getenv("PIXRAIL_API_KEYS"))
	if err != nil {
		return Config{}, err
	}
	if len(keys) == 0 {
		if cfg.RequireConfiguredSecrets {
			return Config{}, fmt.Errorf("PIXRAIL_API_KEYS is required in production")
		}
		keys["dev-secret"] = APIKey{TenantID: "tenant_demo", Secret: "dev-secret"}
	}
	if cfg.StoreDriver != "memory" && cfg.StoreDriver != "postgres" {
		return Config{}, fmt.Errorf("PIXRAIL_STORE_DRIVER must be memory or postgres")
	}
	if cfg.StoreDriver == "postgres" && cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("PIXRAIL_DATABASE_URL is required when PIXRAIL_STORE_DRIVER=postgres")
	}
	if env == "production" && cfg.StoreDriver != "postgres" {
		return Config{}, fmt.Errorf("PIXRAIL_STORE_DRIVER=postgres is required in production")
	}
	cfg.APIKeys = keys
	return cfg, nil
}

func parseAPIKeys(raw string) (map[string]APIKey, error) {
	keys := make(map[string]APIKey)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return keys, nil
	}
	for _, pair := range strings.Split(raw, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("invalid PIXRAIL_API_KEYS entry %q, expected tenant_id:secret", pair)
		}
		tenantID := strings.TrimSpace(parts[0])
		secret := strings.TrimSpace(parts[1])
		keys[secret] = APIKey{TenantID: tenantID, Secret: secret}
	}
	return keys, nil
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
