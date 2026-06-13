package core_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"clipboard/core"
)

func TestThumbnailGeneration(t *testing.T) {
	// 1. Generate test video and image
	testVideo := filepath.Join(os.TempDir(), "clipd_test_video.mp4")
	testImage := filepath.Join(os.TempDir(), "clipd_test_image.png")

	// Ensure clean start
	os.Remove(testVideo)
	os.Remove(testImage)

	// Create test video (1 second long)
	cmdVideo := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "testsrc=duration=1:size=320x240:rate=30", "-pix_fmt", "yuv420p", testVideo)
	if err := cmdVideo.Run(); err != nil {
		t.Fatalf("failed to create test video: %v", err)
	}
	defer os.Remove(testVideo)

	// Create test image
	cmdImage := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "testsrc=duration=1:size=320x240:rate=30", "-vframes", "1", testImage)
	if err := cmdImage.Run(); err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}
	defer os.Remove(testImage)

	// Create a new HistoryStore
	store := core.NewHistoryStore(time.Hour, 10)

	t.Run("Test Image Path", func(t *testing.T) {
		added := store.AddText(testImage)
		if !added {
			t.Fatal("expected image path to be added")
		}

		entries := store.All()
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		entry := entries[0]
		if !entry.IsImage {
			t.Error("expected entry to be marked as Image")
		}
		if entry.IsVideo {
			t.Error("expected entry to not be marked as Video")
		}
		if entry.GetThumbnailPath() != testImage {
			t.Errorf("expected ThumbnailPath to be %q, got %q", testImage, entry.GetThumbnailPath())
		}
	})

	t.Run("Test Video Path", func(t *testing.T) {
		store.Clear()
		added := store.AddText(testVideo)
		if !added {
			t.Fatal("expected video path to be added")
		}

		entries := store.All()
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		entry := entries[0]
		if !entry.IsVideo {
			t.Error("expected entry to be marked as Video")
		}
		if entry.IsImage {
			t.Error("expected entry to not be marked as Image")
		}

		// Wait up to 2 seconds for background thumbnail generation
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if entry.GetThumbnailPath() != "" {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if entry.GetThumbnailPath() == "" {
			t.Fatal("expected ThumbnailPath to be populated in background")
		}

		if _, err := os.Stat(entry.GetThumbnailPath()); err != nil {
			t.Errorf("expected thumbnail file to exist at %q, got error: %v", entry.GetThumbnailPath(), err)
		}
		defer os.Remove(entry.GetThumbnailPath())
	})

	t.Run("Test Raw Image Data", func(t *testing.T) {
		store.Clear()
		imgData, err := os.ReadFile(testImage)
		if err != nil {
			t.Fatalf("failed to read test image: %v", err)
		}

		added := store.AddImage(imgData)
		if !added {
			t.Fatal("expected raw image to be added")
		}

		entries := store.All()
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		entry := entries[0]
		if !entry.IsImage {
			t.Error("expected entry to be marked as Image")
		}
		if entry.IsVideo {
			t.Error("expected entry to not be marked as Video")
		}
		if entry.GetThumbnailPath() == "" {
			t.Fatal("expected ThumbnailPath to be populated for raw image")
		}

		if _, err := os.Stat(entry.GetThumbnailPath()); err != nil {
			t.Errorf("expected raw image thumbnail file to exist at %q, got error: %v", entry.GetThumbnailPath(), err)
		}
		defer os.Remove(entry.GetThumbnailPath())
	})

	t.Run("Test Generic File Path", func(t *testing.T) {
		store.Clear()
		testDoc := filepath.Join(os.TempDir(), "clipd_test_doc.pdf")
		_ = os.WriteFile(testDoc, []byte("dummy pdf content"), 0644)
		defer os.Remove(testDoc)

		added := store.AddText(testDoc)
		if !added {
			t.Fatal("expected file path to be added")
		}

		entries := store.All()
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		entry := entries[0]
		if entry.IsImage {
			t.Error("expected entry to not be marked as Image")
		}
		if entry.IsVideo {
			t.Error("expected entry to not be marked as Video")
		}
		if entry.FilePath != testDoc {
			t.Errorf("expected FilePath to be %q, got %q", testDoc, entry.FilePath)
		}
		if entry.GetThumbnailPath() != "" {
			t.Errorf("expected no ThumbnailPath, got %q", entry.GetThumbnailPath())
		}
	})
}

func TestPersistence(t *testing.T) {
	// Clean start: remove any existing history file
	historyFile := filepath.Join(core.StorageDir(), "history.json")
	os.Remove(historyFile)
	defer os.Remove(historyFile)

	store1 := core.NewHistoryStore(time.Hour, 10)
	store1.AddText("Persisted Text Item 1")
	store1.AddText("Persisted Text Item 2")

	// Verify they are saved
	if _, err := os.Stat(historyFile); err != nil {
		t.Fatalf("history.json was not created: %v", err)
	}

	// Create a new store and load the history
	store2 := core.NewHistoryStore(time.Hour, 10)
	store2.Load()

	entries := store2.All()
	if len(entries) != 2 {
		t.Fatalf("expected 2 loaded entries, got %d", len(entries))
	}

	if entries[0].Content != "Persisted Text Item 2" {
		t.Errorf("expected newest entry to be %q, got %q", "Persisted Text Item 2", entries[0].Content)
	}
	if entries[1].Content != "Persisted Text Item 1" {
		t.Errorf("expected oldest entry to be %q, got %q", "Persisted Text Item 1", entries[1].Content)
	}
}
