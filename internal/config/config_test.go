package config

import "testing"

func TestParseAPIKeys(t *testing.T) {
	keys, err := parseAPIKeys("tenant_a:secret-a,tenant_b:secret-b")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if keys["secret-a"].TenantID != "tenant_a" {
		t.Fatalf("unexpected tenant mapping: %+v", keys["secret-a"])
	}
	if keys["secret-b"].TenantID != "tenant_b" {
		t.Fatalf("unexpected tenant mapping: %+v", keys["secret-b"])
	}
	if !keys["secret-a"].HasRole(RoleTenant) || keys["secret-a"].HasRole(RoleWorker) {
		t.Fatalf("expected legacy key syntax to grant tenant role only: %+v", keys["secret-a"])
	}
}

func TestParseAPIKeysWithRoles(t *testing.T) {
	keys, err := parseAPIKeys("tenant_a:tenant-secret:tenant,tenant_a:worker-secret:worker,tenant_a:ops-secret:worker|provider")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !keys["tenant-secret"].HasRole(RoleTenant) || keys["tenant-secret"].HasRole(RoleWorker) {
		t.Fatalf("expected tenant role only, got %+v", keys["tenant-secret"])
	}
	if !keys["worker-secret"].HasRole(RoleWorker) || keys["worker-secret"].HasRole(RoleProvider) {
		t.Fatalf("expected worker role only, got %+v", keys["worker-secret"])
	}
	if !keys["ops-secret"].HasRole(RoleWorker) || !keys["ops-secret"].HasRole(RoleProvider) {
		t.Fatalf("expected multi-role ops key, got %+v", keys["ops-secret"])
	}
}

func TestParseAPIKeysRejectsMalformedEntries(t *testing.T) {
	if _, err := parseAPIKeys("tenant-only"); err == nil {
		t.Fatal("expected malformed key error")
	}
	if _, err := parseAPIKeys("tenant_a:secret:admin"); err == nil {
		t.Fatal("expected unknown role error")
	}
	if _, err := parseAPIKeys("tenant_a:shared:tenant,tenant_b:shared:tenant"); err == nil {
		t.Fatal("expected duplicate secret error")
	}
}

func TestLoadRequiresAPIKeysInProduction(t *testing.T) {
	t.Setenv("PIXRAIL_ENV", "production")
	t.Setenv("PIXRAIL_API_KEYS", "")
	t.Setenv("PIXRAIL_PROVIDER_CALLBACK_SECRET", "provider-callback-secret")

	if _, err := Load(); err == nil {
		t.Fatal("expected production config to require API keys")
	}
}

func TestLoadRequiresProviderCallbackSecretInProduction(t *testing.T) {
	t.Setenv("PIXRAIL_ENV", "production")
	t.Setenv("PIXRAIL_API_KEYS", "tenant_a:secret-a")
	t.Setenv("PIXRAIL_PROVIDER_CALLBACK_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected production config to require provider callback secret")
	}
}

func TestLoadProvidesDevelopmentKey(t *testing.T) {
	t.Setenv("PIXRAIL_ENV", "development")
	t.Setenv("PIXRAIL_API_KEYS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.APIKeys["dev-secret"].TenantID != "tenant_demo" || !cfg.APIKeys["dev-secret"].HasRole(RoleTenant) {
		t.Fatalf("expected development API key, got %+v", cfg.APIKeys)
	}
	if !cfg.APIKeys["worker-secret"].HasRole(RoleWorker) || !cfg.APIKeys["risk-secret"].HasRole(RoleRisk) || !cfg.APIKeys["provider-secret"].HasRole(RoleProvider) {
		t.Fatalf("expected role-separated development API keys, got %+v", cfg.APIKeys)
	}
	if cfg.ProviderCallbackSecret == "" || cfg.ProviderSignatureTolerance.String() != "5m0s" {
		t.Fatalf("expected development provider callback defaults, got secret=%q tolerance=%s", cfg.ProviderCallbackSecret, cfg.ProviderSignatureTolerance)
	}
}

func TestLoadRejectsProductionMemoryStore(t *testing.T) {
	t.Setenv("PIXRAIL_ENV", "production")
	t.Setenv("PIXRAIL_API_KEYS", "tenant_a:secret-a")
	t.Setenv("PIXRAIL_PROVIDER_CALLBACK_SECRET", "provider-callback-secret")
	t.Setenv("PIXRAIL_STORE_DRIVER", "memory")

	if _, err := Load(); err == nil {
		t.Fatal("expected production to require postgres store")
	}
}

func TestLoadRequiresDatabaseURLForPostgres(t *testing.T) {
	t.Setenv("PIXRAIL_ENV", "development")
	t.Setenv("PIXRAIL_STORE_DRIVER", "postgres")
	t.Setenv("PIXRAIL_DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected postgres store to require database URL")
	}
}

func TestLoadTracingExporterDefaults(t *testing.T) {
	t.Setenv("PIXRAIL_ENV", "development")
	t.Setenv("PIXRAIL_API_KEYS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.TracingExporter != "stdout" {
		t.Fatalf("expected development tracing stdout, got %q", cfg.TracingExporter)
	}
	if cfg.WorkerBatchSize != 100 || cfg.WorkerInterval.String() != "1s" {
		t.Fatalf("unexpected worker defaults: batch=%d interval=%s", cfg.WorkerBatchSize, cfg.WorkerInterval)
	}
	if cfg.PostgresMaxConns != 10 || cfg.PostgresMinConns != 0 || cfg.PostgresMaxConnLifetime.String() != "30m0s" {
		t.Fatalf("unexpected postgres pool defaults: min=%d max=%d lifetime=%s", cfg.PostgresMinConns, cfg.PostgresMaxConns, cfg.PostgresMaxConnLifetime)
	}

	t.Setenv("PIXRAIL_ENV", "production")
	t.Setenv("PIXRAIL_API_KEYS", "tenant_a:secret-a")
	t.Setenv("PIXRAIL_STORE_DRIVER", "postgres")
	t.Setenv("PIXRAIL_DATABASE_URL", "postgres://example")
	t.Setenv("PIXRAIL_TRACING_EXPORTER", "")
	t.Setenv("PIXRAIL_PROVIDER_CALLBACK_SECRET", "provider-callback-secret")

	cfg, err = Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.TracingExporter != "none" {
		t.Fatalf("expected production tracing none, got %q", cfg.TracingExporter)
	}
}

func TestLoadRejectsUnknownTracingExporter(t *testing.T) {
	t.Setenv("PIXRAIL_TRACING_EXPORTER", "debug")

	if _, err := Load(); err == nil {
		t.Fatal("expected unknown tracing exporter to fail")
	}
}

func TestLoadRejectsMalformedIntegerRuntimeConfig(t *testing.T) {
	t.Setenv("PIXRAIL_WORKER_BATCH_SIZE", "abc")

	if _, err := Load(); err == nil {
		t.Fatal("expected malformed integer runtime config to fail")
	}
}

func TestLoadRejectsNonPositiveRuntimeConfig(t *testing.T) {
	t.Setenv("PIXRAIL_WORKER_BATCH_SIZE", "0")

	if _, err := Load(); err == nil {
		t.Fatal("expected non-positive worker batch size to fail")
	}
}

func TestLoadRejectsMalformedDurationRuntimeConfig(t *testing.T) {
	t.Setenv("PIXRAIL_WORKER_INTERVAL", "later")

	if _, err := Load(); err == nil {
		t.Fatal("expected malformed duration runtime config to fail")
	}
}

func TestLoadAllowsZeroPostgresMinConns(t *testing.T) {
	t.Setenv("PIXRAIL_POSTGRES_MIN_CONNS", "0")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.PostgresMinConns != 0 {
		t.Fatalf("expected postgres min conns to remain zero, got %d", cfg.PostgresMinConns)
	}
}
