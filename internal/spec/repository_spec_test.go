package spec

import (
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
