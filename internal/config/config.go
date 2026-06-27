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
	Roles    map[APIKeyRole]bool
}

type APIKeyRole string

const (
	RoleTenant   APIKeyRole = "tenant"
	RoleWorker   APIKeyRole = "worker"
	RoleRisk     APIKeyRole = "risk"
	RoleProvider APIKeyRole = "provider"
)

func (k APIKey) HasRole(role APIKeyRole) bool {
	if len(k.Roles) == 0 {
		return role == RoleTenant
	}
	return k.Roles[role]
}

type Config struct {
	Addr                       string
	Environment                string
	APIKeys                    map[string]APIKey
	StoreDriver                string
	DatabaseURL                string
	PostgresMinConns           int
	PostgresMaxConns           int
	PostgresMaxConnLifetime    time.Duration
	TenantBucketSize           int
	DictBucketSize             int
	DictTimeout                time.Duration
	WorkerBatchSize            int
	WorkerInterval             time.Duration
	ProviderCallbackSecret     string
	ProviderSignatureTolerance time.Duration
	PprofAddr                  string
	ShutdownTimeout            time.Duration
	RequireConfiguredSecrets   bool
	TracingExporter            string
}

func Load() (Config, error) {
	env := getenv("PIXRAIL_ENV", "development")
	postgresMinConns, err := getenvIntAtLeast("PIXRAIL_POSTGRES_MIN_CONNS", 0, 0)
	if err != nil {
		return Config{}, err
	}
	postgresMaxConns, err := getenvIntAtLeast("PIXRAIL_POSTGRES_MAX_CONNS", 10, 1)
	if err != nil {
		return Config{}, err
	}
	postgresMaxConnLifetime, err := getenvDurationAtLeast("PIXRAIL_POSTGRES_MAX_CONN_LIFETIME", 30*time.Minute, time.Nanosecond)
	if err != nil {
		return Config{}, err
	}
	tenantBucketSize, err := getenvIntAtLeast("PIXRAIL_TENANT_BUCKET_SIZE", 120, 1)
	if err != nil {
		return Config{}, err
	}
	dictBucketSize, err := getenvIntAtLeast("PIXRAIL_DICT_BUCKET_SIZE", 60, 1)
	if err != nil {
		return Config{}, err
	}
	dictTimeout, err := getenvDurationAtLeast("PIXRAIL_DICT_TIMEOUT", 300*time.Millisecond, time.Nanosecond)
	if err != nil {
		return Config{}, err
	}
	workerBatchSize, err := getenvIntAtLeast("PIXRAIL_WORKER_BATCH_SIZE", 100, 1)
	if err != nil {
		return Config{}, err
	}
	workerInterval, err := getenvDurationAtLeast("PIXRAIL_WORKER_INTERVAL", time.Second, time.Nanosecond)
	if err != nil {
		return Config{}, err
	}
	providerSignatureTolerance, err := getenvDurationAtLeast("PIXRAIL_PROVIDER_SIGNATURE_TOLERANCE", 5*time.Minute, time.Nanosecond)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := getenvDurationAtLeast("PIXRAIL_SHUTDOWN_TIMEOUT", 5*time.Second, time.Nanosecond)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Addr:                       getenv("PIXRAIL_HTTP_ADDR", ":8080"),
		Environment:                env,
		StoreDriver:                getenv("PIXRAIL_STORE_DRIVER", "memory"),
		DatabaseURL:                getenv("PIXRAIL_DATABASE_URL", ""),
		PostgresMinConns:           postgresMinConns,
		PostgresMaxConns:           postgresMaxConns,
		PostgresMaxConnLifetime:    postgresMaxConnLifetime,
		TenantBucketSize:           tenantBucketSize,
		DictBucketSize:             dictBucketSize,
		DictTimeout:                dictTimeout,
		WorkerBatchSize:            workerBatchSize,
		WorkerInterval:             workerInterval,
		ProviderCallbackSecret:     getenv("PIXRAIL_PROVIDER_CALLBACK_SECRET", defaultProviderCallbackSecret(env)),
		ProviderSignatureTolerance: providerSignatureTolerance,
		PprofAddr:                  getenv("PIXRAIL_PPROF_ADDR", ""),
		ShutdownTimeout:            shutdownTimeout,
		RequireConfiguredSecrets:   env == "production",
		TracingExporter:            getenv("PIXRAIL_TRACING_EXPORTER", defaultTracingExporter(env)),
	}

	keys, err := parseAPIKeys(os.Getenv("PIXRAIL_API_KEYS"))
	if err != nil {
		return Config{}, err
	}
	if len(keys) == 0 {
		if cfg.RequireConfiguredSecrets {
			return Config{}, fmt.Errorf("PIXRAIL_API_KEYS is required in production")
		}
		keys["dev-secret"] = APIKey{TenantID: "tenant_demo", Secret: "dev-secret", Roles: roleSet(RoleTenant)}
		keys["worker-secret"] = APIKey{TenantID: "tenant_demo", Secret: "worker-secret", Roles: roleSet(RoleWorker)}
		keys["risk-secret"] = APIKey{TenantID: "tenant_demo", Secret: "risk-secret", Roles: roleSet(RoleRisk)}
		keys["provider-secret"] = APIKey{TenantID: "tenant_demo", Secret: "provider-secret", Roles: roleSet(RoleProvider)}
	}
	if cfg.RequireConfiguredSecrets && cfg.ProviderCallbackSecret == "" {
		return Config{}, fmt.Errorf("PIXRAIL_PROVIDER_CALLBACK_SECRET is required in production")
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
	if cfg.TracingExporter != "none" && cfg.TracingExporter != "stdout" {
		return Config{}, fmt.Errorf("PIXRAIL_TRACING_EXPORTER must be none or stdout")
	}
	cfg.APIKeys = keys
	return cfg, nil
}

func defaultTracingExporter(env string) string {
	if env == "production" {
		return "none"
	}
	return "stdout"
}

func defaultProviderCallbackSecret(env string) string {
	if env == "production" {
		return ""
	}
	return "dev-provider-callback-secret"
}

func parseAPIKeys(raw string) (map[string]APIKey, error) {
	keys := make(map[string]APIKey)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return keys, nil
	}
	for _, pair := range strings.Split(raw, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 3)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			if len(parts) != 3 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
				return nil, fmt.Errorf("invalid PIXRAIL_API_KEYS entry %q, expected tenant_id:secret[:role|role]", pair)
			}
		}
		roles := roleSet(RoleTenant)
		if len(parts) == 3 {
			parsedRoles, err := parseRoles(parts[2])
			if err != nil {
				return nil, err
			}
			roles = parsedRoles
		}
		tenantID := strings.TrimSpace(parts[0])
		secret := strings.TrimSpace(parts[1])
		if _, exists := keys[secret]; exists {
			return nil, fmt.Errorf("duplicate PIXRAIL_API_KEYS secret for tenant %q", tenantID)
		}
		keys[secret] = APIKey{TenantID: tenantID, Secret: secret, Roles: roles}
	}
	return keys, nil
}

func parseRoles(raw string) (map[APIKeyRole]bool, error) {
	roles := make(map[APIKeyRole]bool)
	for _, role := range strings.Split(raw, "|") {
		switch parsed := APIKeyRole(strings.TrimSpace(role)); parsed {
		case RoleTenant, RoleWorker, RoleRisk, RoleProvider:
			roles[parsed] = true
		default:
			return nil, fmt.Errorf("invalid PIXRAIL_API_KEYS role %q", role)
		}
	}
	if len(roles) == 0 {
		return nil, fmt.Errorf("PIXRAIL_API_KEYS role list cannot be empty")
	}
	return roles, nil
}

func roleSet(roles ...APIKeyRole) map[APIKeyRole]bool {
	set := make(map[APIKeyRole]bool, len(roles))
	for _, role := range roles {
		set[role] = true
	}
	return set
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getenvIntAtLeast(key string, fallback int, min int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	if parsed < min {
		return 0, fmt.Errorf("%s must be >= %d", key, min)
	}
	return parsed, nil
}

func getenvDurationAtLeast(key string, fallback time.Duration, min time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	if parsed < min {
		return 0, fmt.Errorf("%s must be >= %s", key, min)
	}
	return parsed, nil
}
