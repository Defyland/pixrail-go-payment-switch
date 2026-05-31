package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositorySpecRequiredArtifacts(t *testing.T) {
	root := "../.."
	required := []string{
		"README.md",
		"openapi.yaml",
		"Dockerfile",
		"compose.yaml",
		"docker-compose.yml",
		".github/workflows/ci.yml",
		"docs/spec-driven/senior-readiness-spec.md",
		"docs/spec-driven/implementation-plan.md",
		"docs/spec-driven/verification-report.md",
		"docs/engineering-case-study.md",
		"docs/product/problem.md",
		"docs/product/personas.md",
		"docs/product/use-cases.md",
		"docs/product/non-goals.md",
		"docs/product/roadmap.md",
		"docs/product/pricing-or-plans.md",
		"docs/domain/glossary.md",
		"docs/domain/bounded-contexts.md",
		"docs/domain/aggregates.md",
		"docs/domain/invariants.md",
		"docs/domain/state-machines.md",
		"docs/adr",
		"docs/architecture",
		"docs/architecture/c4-context.md",
		"docs/architecture/c4-container.md",
		"docs/architecture/module-boundaries.md",
		"docs/architecture/sequence-diagrams.md",
		"docs/architecture/deployment-view.md",
		"docs/benchmarks",
		"docs/api",
		"docs/diagrams",
		"docs/events",
		"docs/runbooks",
		"docs/security",
		"docs/security/data-classification.md",
		"docs/security/secrets.md",
		"docs/security/abuse-cases.md",
		"docs/scalability.md",
		"docs/operational-cost.md",
		"cmd/pixrail-migrate/main.go",
		"cmd/pixrail-worker/main.go",
		"db/migrations/0001_pixrail_core.sql",
		"db/migrations/0002_consistency_hardening.sql",
		"db/migrations/0003_worker_leases.sql",
		"benchmarks/baseline.md",
		"benchmarks/k6/smoke.js",
		"benchmarks/k6/load.js",
		"benchmarks/k6/stress.js",
		"benchmarks/k6/spike.js",
		"observability/grafana/pixrail-overview-dashboard.json",
	}
	for _, item := range required {
		if _, err := os.Stat(filepath.Join(root, item)); err != nil {
			t.Fatalf("required artifact %s missing: %v", item, err)
		}
	}
}

func TestReadmeContainsRequiredSections(t *testing.T) {
	raw, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	readme := strings.ToLower(string(raw))
	sections := []string{
		"what is this product?",
		"problem it solves",
		"target users",
		"main features",
		"architecture overview",
		"tech stack",
		"domain model",
		"api documentation",
		"async or event architecture",
		"database design",
		"testing strategy",
		"performance benchmarks",
		"observability",
		"security considerations",
		"trade-offs and decisions",
		"how to run locally",
		"how to run tests",
		"failure scenarios",
		"roadmap",
	}
	for _, section := range sections {
		if !strings.Contains(readme, section) {
			t.Fatalf("README missing section %q", section)
		}
	}
}

func TestEventSchemasUseRequiredEnvelopeFields(t *testing.T) {
	matches, err := filepath.Glob("../../docs/events/*.v1.json")
	if err != nil {
		t.Fatalf("glob schemas: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected event schemas")
	}
	required := []string{"event_id", "event_type", "schema_version", "occurred_at", "producer", "tenant_id", "account_id", "pix_transfer_id", "correlation_id", "payload"}
	for _, match := range matches {
		raw, err := os.ReadFile(match)
		if err != nil {
			t.Fatalf("read schema %s: %v", match, err)
		}
		schema := string(raw)
		for _, field := range required {
			if !strings.Contains(schema, field) {
				t.Fatalf("schema %s missing envelope field %s", match, field)
			}
		}
	}
}

func TestEventSchemasDeclareStrictPayloadContracts(t *testing.T) {
	root := "../.."
	expectedRequired := map[string][]string{
		"dict_key_resolved.v1.json":             {"receiver_id", "receiver_bank", "risk_signal"},
		"fraud_score_calculated.v1.json":        {"score", "status", "rules"},
		"pix_transfer_accepted.v1.json":         {"decision_reason"},
		"pix_transfer_approved.v1.json":         {"end_to_end_id", "spi_message_id", "decision_reason"},
		"pix_transfer_blocked.v1.json":          {"score", "rules", "decision_reason"},
		"pix_transfer_rejected.v1.json":         {"spi_message_id", "status", "code"},
		"pix_transfer_requested.v1.json":        {"amount_cents", "currency", "receiver_key_type"},
		"pix_transfer_review_approved.v1.json":  {"decision", "reason"},
		"pix_transfer_review_blocked.v1.json":   {"decision", "reason"},
		"pix_transfer_review_requested.v1.json": {"score", "rules", "decision_reason"},
		"pix_transfer_settled.v1.json":          {"spi_message_id", "status", "code"},
		"spi_message_created.v1.json":           {"spi_message_id", "end_to_end_id"},
		"spi_submission_requested.v1.json":      {"request_hash"},
	}

	for file, requiredFields := range expectedRequired {
		raw, err := os.ReadFile(filepath.Join(root, "docs/events", file))
		if err != nil {
			t.Fatalf("read schema %s: %v", file, err)
		}
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("schema %s is invalid json: %v", file, err)
		}
		properties := schema["properties"].(map[string]any)
		payload := properties["payload"].(map[string]any)
		if payload["additionalProperties"] != false {
			t.Fatalf("schema %s payload must reject undeclared fields", file)
		}
		required, ok := payload["required"].([]any)
		if !ok || len(required) == 0 {
			t.Fatalf("schema %s payload must declare required fields", file)
		}
		requiredSet := make(map[string]bool, len(required))
		for _, field := range required {
			requiredSet[field.(string)] = true
		}
		for _, field := range requiredFields {
			if !requiredSet[field] {
				t.Fatalf("schema %s payload missing required field %s", file, field)
			}
		}
	}
}
