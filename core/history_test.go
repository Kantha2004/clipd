package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestNormalizeFilePaths guards multi-file clipboard copies: a file manager
// copying several files puts one file:// URI per line, and all of them must
// survive normalization, not just the first.
func TestNormalizeFilePaths(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	content := "file://" + a + "\nfile://" + b
	got := NormalizeFilePaths(content)
	want := []string{a, b}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeFilePaths(%q) = %v, want %v", content, got, want)
	}

	if got := NormalizeFilePath(content); got != a {
		t.Fatalf("NormalizeFilePath(%q) = %q, want %q (first path)", content, got, a)
	}

	e := &Entry{Content: content, FilePath: a}
	if want, got := "📁 2 Files: a.txt", e.Preview(); got != want {
		t.Fatalf("Preview() = %q, want %q", got, want)
	}
}
