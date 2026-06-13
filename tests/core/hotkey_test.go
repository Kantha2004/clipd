package core_test

import (
	"os"
	"testing"

	"clipboard/core"
)

func TestReadPrevWin(t *testing.T) {
	// Save and restore PrevWinFile around all sub-tests.
	original := core.PrevWinFile
	t.Cleanup(func() { core.PrevWinFile = original })

	t.Run("missing file returns empty string", func(t *testing.T) {
		// Point PrevWinFile at a path that does not exist.
		core.PrevWinFile = "/tmp/clipd_prevwin_nonexistent_test_xyz"
		os.Remove(core.PrevWinFile) // ensure it really is absent

		got := core.ReadPrevWin()
		if got != "" {
			t.Errorf("ReadPrevWin() = %q; want %q", got, "")
		}
	})

	t.Run("file with trailing whitespace returns trimmed value", func(t *testing.T) {
		// Write a window ID with surrounding whitespace into a temp file.
		tmpFile, err := os.CreateTemp("", "clipd_prevwin_test_*")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		windowID := "0x03a00007"
		if _, err := tmpFile.WriteString(windowID + "  \n"); err != nil {
			t.Fatalf("failed to write to temp file: %v", err)
		}
		tmpFile.Close()

		core.PrevWinFile = tmpFile.Name()

		got := core.ReadPrevWin()
		if got != windowID {
			t.Errorf("ReadPrevWin() = %q; want %q", got, windowID)
		}
	})
}
