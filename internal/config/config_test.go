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
}

func TestParseAPIKeysRejectsMalformedEntries(t *testing.T) {
	if _, err := parseAPIKeys("tenant-only"); err == nil {
		t.Fatal("expected malformed key error")
	}
}

func TestLoadRequiresAPIKeysInProduction(t *testing.T) {
	t.Setenv("PIXRAIL_ENV", "production")
	t.Setenv("PIXRAIL_API_KEYS", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected production config to require API keys")
	}
}

func TestLoadProvidesDevelopmentKey(t *testing.T) {
	t.Setenv("PIXRAIL_ENV", "development")
	t.Setenv("PIXRAIL_API_KEYS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.APIKeys["dev-secret"].TenantID != "tenant_demo" {
		t.Fatalf("expected development API key, got %+v", cfg.APIKeys)
	}
}

func TestLoadRejectsProductionMemoryStore(t *testing.T) {
	t.Setenv("PIXRAIL_ENV", "production")
	t.Setenv("PIXRAIL_API_KEYS", "tenant_a:secret-a")
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

	t.Setenv("PIXRAIL_ENV", "production")
	t.Setenv("PIXRAIL_API_KEYS", "tenant_a:secret-a")
	t.Setenv("PIXRAIL_STORE_DRIVER", "postgres")
	t.Setenv("PIXRAIL_DATABASE_URL", "postgres://example")
	t.Setenv("PIXRAIL_TRACING_EXPORTER", "")

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
