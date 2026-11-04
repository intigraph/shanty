package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Linker -X silently ignores an old package path. Keep every build surface
// aligned with the module so release version metadata cannot vanish unnoticed.
func TestBuildRecipesUseModuleIdentity(t *testing.T) {
	root := filepath.Join("..", "..")
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(mod))
	if len(fields) < 2 || fields[0] != "module" {
		t.Fatal("missing module declaration")
	}
	pattern := regexp.MustCompile(`-X\s+([^=\s]+)=`)
	for _, recipe := range []string{"Makefile", "justfile", ".goreleaser.yml"} {
		data, err := os.ReadFile(filepath.Join(root, recipe))
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]int{}
		for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
			found[match[1]]++
		}
		if len(found) != 3 {
			t.Fatalf("%s: expected three metadata assignments, got %v", recipe, found)
		}
		for _, field := range []string{"Version", "Commit", "BuildTime"} {
			key := fields[1] + "/internal/cmd." + field
			if found[key] != 1 {
				t.Errorf("%s: expected one %s assignment, got %v", recipe, key, found)
			}
		}
	}
}
