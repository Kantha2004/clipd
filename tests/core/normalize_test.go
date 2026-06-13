package core_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"clipboard/core"
)

func TestNormalizeFilePath(t *testing.T) {
	// Create a real temp file for cases that require an existing file.
	tmpFile, err := os.CreateTemp("", "clipd_normalize_test_*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Create a real temp file whose name contains a space (via symlink-free rename).
	tmpFileSpace, err := os.CreateTemp("", "clipd normalize space*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file with space: %v", err)
	}
	tmpFileSpace.Close()
	defer os.Remove(tmpFileSpace.Name())

	// Create a real temp directory for the "is a directory" case.
	tmpDir, err := os.MkdirTemp("", "clipd_normalize_dir_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Encode the space-containing path as a %20 URL so we can verify decoding.
	encoded := func(path string) string {
		return strings.ReplaceAll(path, " ", "%20")
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "relative path",
			input: "foo/bar.txt",
			want:  "",
		},
		{
			name:  "nonexistent file with file:// prefix",
			input: "file:///tmp/nonexistent_clipd_xyz.txt",
			want:  "",
		},
		{
			name:  "directory path returns empty",
			input: tmpDir,
			want:  "",
		},
		{
			name:  "valid absolute file path",
			input: tmpFile.Name(),
			want:  tmpFile.Name(),
		},
		{
			name:  "file:// prefix stripped",
			input: "file://" + tmpFile.Name(),
			want:  tmpFile.Name(),
		},
		{
			name:  "percent-encoded spaces decoded",
			input: encoded(tmpFileSpace.Name()),
			want:  tmpFileSpace.Name(),
		},
		{
			name:  "multi-line content uses only first line",
			input: fmt.Sprintf("%s\n/some/other/path\n", tmpFile.Name()),
			want:  tmpFile.Name(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := core.NormalizeFilePath(tc.input)
			if got != tc.want {
				t.Errorf("NormalizeFilePath(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}
