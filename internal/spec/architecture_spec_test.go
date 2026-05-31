package spec

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const modulePath = "github.com/Defyland/pixrail-go-payment-switch"

func TestArchitectureDependencyRule(t *testing.T) {
	imports := productionImports(t)

	assertNoInternalImports(t, imports, "internal/rail")
	assertNoInternalImports(t, imports, "internal/events")

	assertOnlyInternalImports(t, imports, "internal/switcher", []string{
		"internal/events",
		"internal/rail",
	})

	assertDoesNotImport(t, imports, "internal/api", []string{
		"internal/app",
		"internal/postgres",
		"internal/store",
		"internal/dict",
		"internal/fraud",
		"internal/spi",
		"internal/ratelimit",
		"internal/codec",
	})

	for _, adapter := range []string{"internal/postgres", "internal/store", "internal/dict", "internal/spi"} {
		assertDoesNotImport(t, imports, adapter, []string{
			"internal/api",
			"internal/app",
			"internal/switcher",
			"internal/config",
			"internal/observability",
		})
	}
}

func TestArchitectureDocsExplainNotMVC(t *testing.T) {
	root := "../.."
	required := map[string][]string{
		"docs/architecture/ports-and-adapters.md": {"primary adapters", "use cases", "output ports", "adapters"},
		"docs/architecture/go-architecture.md":    {"modular monolith", "not mvc", "framework at the edge"},
		"docs/architecture/dependency-rule.md":    {"dependency rule", "internal/switcher", "internal/rail"},
		"docs/architecture/testing-strategy.md":   {"domain tests", "use case tests", "adapter tests"},
		"docs/architecture/module-boundaries.md":  {"payment switch", "ports", "adapters"},
	}
	for path, phrases := range required {
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := strings.ToLower(string(raw))
		for _, phrase := range phrases {
			if !strings.Contains(text, phrase) {
				t.Fatalf("%s must mention %q", path, phrase)
			}
		}
	}
}

func productionImports(t *testing.T) map[string][]string {
	t.Helper()
	root := "../.."
	imports := make(map[string][]string)
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".gocache":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		dir := filepath.Dir(path)
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(rel)
		if pkg == "." {
			return nil
		}
		for _, importSpec := range parsed.Imports {
			importPath := strings.Trim(importSpec.Path.Value, `"`)
			if strings.HasPrefix(importPath, modulePath+"/internal/") {
				imports[pkg] = append(imports[pkg], strings.TrimPrefix(importPath, modulePath+"/"))
			}
		}
		slices.Sort(imports[pkg])
		imports[pkg] = slices.Compact(imports[pkg])
		return nil
	})
	if err != nil {
		t.Fatalf("walk production imports: %v", err)
	}
	return imports
}

func assertNoInternalImports(t *testing.T, imports map[string][]string, pkg string) {
	t.Helper()
	if got := imports[pkg]; len(got) > 0 {
		t.Fatalf("%s must not import internal packages, got %v", pkg, got)
	}
}

func assertOnlyInternalImports(t *testing.T, imports map[string][]string, pkg string, allowed []string) {
	t.Helper()
	allowedSet := make(map[string]bool, len(allowed))
	for _, item := range allowed {
		allowedSet[item] = true
	}
	for _, importPath := range imports[pkg] {
		if !allowedSet[importPath] {
			t.Fatalf("%s imports forbidden internal package %s; allowed=%v", pkg, importPath, allowed)
		}
	}
}

func assertDoesNotImport(t *testing.T, imports map[string][]string, pkg string, forbidden []string) {
	t.Helper()
	forbiddenSet := make(map[string]bool, len(forbidden))
	for _, item := range forbidden {
		forbiddenSet[item] = true
	}
	for _, importPath := range imports[pkg] {
		if forbiddenSet[importPath] {
			t.Fatalf("%s imports forbidden internal package %s", pkg, importPath)
		}
	}
}
