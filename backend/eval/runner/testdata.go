package runner

import (
	"os"
	"path/filepath"
	"testing"

	"sthira/backend/eval/corpus"
)

// loadBuiltIn loads the committed synthetic suite from the
// testdata dir. Used to keep the runner self-contained in CI without
// relying on the eval-run binary being in PATH.
func loadBuiltIn(t *testing.T) corpus.Suite {
	t.Helper()
	paths := []string{
		filepath.Join("..", "cases", "synthetic", "suite.json"),
		filepath.Join("cases", "synthetic", "suite.json"),
	}
	for _, p := range paths {
		data, err := os.ReadFile(filepath.Clean(p)) // #nosec G304
		if err != nil {
			continue
		}
		suite, err := corpus.Parse(data)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		return suite
	}
	t.Fatalf("no synthetic suite present")
	return corpus.Suite{}
}
