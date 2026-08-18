package ui

import "testing"

// TestBuildURIList guards the multi-file copy fix: writeToClipboard must
// emit one file:// URI per path, not just the first.
func TestBuildURIList(t *testing.T) {
	got := buildURIList([]string{"/a/one.txt", "/a/two.txt"})
	want := "file:///a/one.txt\r\nfile:///a/two.txt\r\n"
	if got != want {
		t.Fatalf("buildURIList() = %q, want %q", got, want)
	}
}
