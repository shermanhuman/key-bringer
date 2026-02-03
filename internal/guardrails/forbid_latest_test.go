package guardrails

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForbidVersionsLatest(t *testing.T) {
	root := repoRoot(t)
	paths := []string{filepath.Join(root, "internal"), filepath.Join(root, "cmd")}
	forbidden := "versions/" + "latest"

	for _, base := range paths {
		_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(b), forbidden) {
				t.Fatalf("forbidden string found in %s", path)
			}
			return nil
		})
	}
}

func repoRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root")
		}
		dir = parent
	}
}
