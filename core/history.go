package core

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const previewMaxRunes = 100

// Entry is a single clipboard snapshot.
type Entry struct {
	mu            sync.RWMutex
	Content       string
	Timestamp     time.Time
	IsImage       bool
	IsVideo       bool
	ImageData     []byte // For raw clipboard images
	FilePath      string // For file paths to images/videos
	thumbnailPath string // Path to the generated thumbnail on disk
}

func (e *Entry) GetThumbnailPath() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.thumbnailPath
}

func (e *Entry) SetThumbnailPath(p string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.thumbnailPath = p
}

type jsonEntry struct {
	Content       string    `json:"content"`
	Timestamp     time.Time `json:"timestamp"`
	IsImage       bool      `json:"is_image"`
	IsVideo       bool      `json:"is_video"`
	ImageData     []byte    `json:"image_data,omitempty"`
	FilePath      string    `json:"file_path,omitempty"`
	ThumbnailPath string    `json:"thumbnail_path,omitempty"`
}

func (e *Entry) MarshalJSON() ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	je := jsonEntry{
		Content:       e.Content,
		Timestamp:     e.Timestamp,
		IsImage:       e.IsImage,
		IsVideo:       e.IsVideo,
		ImageData:     e.ImageData,
		FilePath:      e.FilePath,
		ThumbnailPath: e.thumbnailPath,
	}
	return json.Marshal(je)
}

func (e *Entry) UnmarshalJSON(data []byte) error {
	var je jsonEntry
	if err := json.Unmarshal(data, &je); err != nil {
		return err
	}
	e.Content = je.Content
	e.Timestamp = je.Timestamp
	e.IsImage = je.IsImage
	e.IsVideo = je.IsVideo
	e.ImageData = je.ImageData
	e.FilePath = je.FilePath
	e.thumbnailPath = je.ThumbnailPath
	return nil
}

func (e *Entry) cleanupMedia() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.thumbnailPath != "" {
		mDir := mediaDir()
		if strings.HasPrefix(e.thumbnailPath, mDir) {
			if err := os.Remove(e.thumbnailPath); err == nil {
				log.Printf("Removed generated media file: %s", e.thumbnailPath)
			} else if !os.IsNotExist(err) {
				log.Printf("Error removing generated media file %s: %v", e.thumbnailPath, err)
			}
		}
	}
}

func StorageDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "clipd")
	}
	dir := filepath.Join(home, ".config", "clipd")
	os.MkdirAll(dir, 0755)
	return dir
}

func mediaDir() string {
	dir := filepath.Join(StorageDir(), "media")
	os.MkdirAll(dir, 0755)
	return dir
}

// Preview returns a display-safe truncated string (max 4 lines).
func (e *Entry) Preview() string {
	if e.IsImage && e.FilePath == "" {
		return "📋 Image Copy"
	}
	if e.IsImage && e.FilePath != "" {
		return "🖼️ Image File: " + filepath.Base(e.FilePath)
	}
	if e.IsVideo {
		return "🎥 Video File: " + filepath.Base(e.FilePath)
	}
	if e.FilePath != "" {
		return "📄 File: " + filepath.Base(e.FilePath)
	}

	lines := strings.SplitN(e.Content, "\n", 5)
	if len(lines) > 4 {
		lines = append(lines[:4], "…")
	}
	s := strings.Join(lines, "\n")
	if utf8.RuneCountInString(s) > previewMaxRunes {
		runes := []rune(s)
		return string(runes[:previewMaxRunes]) + "…"
	}
	return s
}

var (
	imgExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true}
	vidExts = map[string]bool{".mp4": true, ".mkv": true, ".webm": true, ".avi": true, ".mov": true, ".flv": true, ".wmv": true}
)

// ProcessThumbnail checks if content represents a path to an image/video and generates a thumbnail.
func (e *Entry) ProcessThumbnail(onComplete func()) {
	path := NormalizeFilePath(e.Content)
	if path == "" {
		return
	}
	e.FilePath = path
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case imgExts[ext]:
		e.IsImage = true
		e.SetThumbnailPath(path)
	case vidExts[ext]:
		e.IsVideo = true
		thumbPath := filepath.Join(mediaDir(), fmt.Sprintf("%x", md5.Sum([]byte(path)))+".jpg")
		if _, err := os.Stat(thumbPath); os.IsNotExist(err) {
			go e.generateVideoThumb(path, thumbPath, onComplete)
		} else {
			e.SetThumbnailPath(thumbPath)
		}
	}
}

// normalizeFilePath extracts and validates an absolute file path from clipboard content.
func NormalizeFilePath(content string) string {
	s := strings.TrimSpace(content)
	s = strings.TrimPrefix(s, "file://")
	s = strings.ReplaceAll(s, "%20", " ")
	line, _, _ := strings.Cut(s, "\n")
	line = strings.TrimSpace(line)
	if !filepath.IsAbs(line) {
		return ""
	}
	stat, err := os.Stat(line)
	if err != nil || stat.IsDir() {
		return ""
	}
	return line
}

// generateVideoThumb extracts a frame from src into thumbPath, trying 1s then 0s as offsets.
func (e *Entry) generateVideoThumb(src, thumbPath string, onComplete func()) {
	for _, offset := range []string{"00:00:01", "00:00:00"} {
		if extractFrame(src, thumbPath, offset) {
			e.SetThumbnailPath(thumbPath)
			if onComplete != nil {
				onComplete()
			}
			return
		}
		_ = os.Remove(thumbPath) // clean up any empty/partial file before retry
	}
}

// extractFrame runs ffmpeg to pull a single frame at offset and returns true if the output file is non-empty.
func extractFrame(src, dst, offset string) bool {
	err := exec.Command("ffmpeg", "-y", "-ss", offset, "-i", src, "-vframes", "1", "-vf", "scale=120:-1", "-q:v", "5", dst).Run()
	if err != nil {
		return false
	}
	stat, err := os.Stat(dst)
	return err == nil && stat.Size() > 0
}

// HistoryStore holds clipboard entries newest-first with bounded size and age.
type HistoryStore struct {
	mu        sync.RWMutex
	entries   []*Entry
	maxAge    time.Duration
	maxSize   int
	onChanged func()
}

func NewHistoryStore(maxAge time.Duration, maxSize int) *HistoryStore {
	return &HistoryStore{maxAge: maxAge, maxSize: maxSize}
}

func (h *HistoryStore) SetOnChanged(cb func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onChanged = cb
}

// AddText inserts content at the front, deduplicates, and evicts stale entries.
func (h *HistoryStore) AddText(content string) bool {
	if content == "" {
		return false
	}
	h.mu.Lock()
	if len(h.entries) > 0 && h.entries[0].Content == content && !h.entries[0].IsImage {
		h.mu.Unlock()
		return false
	}

	// Remove duplicate text entries
	for i, e := range h.entries {
		if e.Content == content && !e.IsImage {
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			break
		}
	}
	h.mu.Unlock()

	h.mu.RLock()
	cb := h.onChanged
	h.mu.RUnlock()

	entry := &Entry{Content: content, Timestamp: time.Now()}
	entry.ProcessThumbnail(func() {
		h.save()
		if cb != nil {
			cb()
		}
	})

	h.mu.Lock()
	h.entries = append([]*Entry{entry}, h.entries...)
	h.evict()
	h.mu.Unlock()

	h.save()

	if cb != nil {
		cb()
	}
	return true
}

// AddImage saves raw image bytes from the clipboard, generates a thumbnail path, and evicts stale entries.
func (h *HistoryStore) AddImage(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	h.mu.Lock()
	// Check if same as last image
	if len(h.entries) > 0 && h.entries[0].IsImage && bytes.Equal(h.entries[0].ImageData, data) {
		h.mu.Unlock()
		return false
	}

	// Save to temp file
	thumbDir := mediaDir()

	hash := fmt.Sprintf("%x", md5.Sum(data))
	imgPath := filepath.Join(thumbDir, hash+".png")
	_ = os.WriteFile(imgPath, data, 0644)

	entry := &Entry{
		Content:       "[Image Data]",
		Timestamp:     time.Now(),
		IsImage:       true,
		ImageData:     data,
		thumbnailPath: imgPath,
	}

	h.entries = append([]*Entry{entry}, h.entries...)
	h.evict()
	cb := h.onChanged
	h.mu.Unlock()

	h.save()

	if cb != nil {
		cb()
	}
	return true
}

func (h *HistoryStore) evict() {
	cutoff := time.Now().Add(-h.maxAge)
	n := len(h.entries)
	for i, e := range h.entries {
		if e.Timestamp.Before(cutoff) {
			n = i
			break
		}
	}
	if n > h.maxSize {
		n = h.maxSize
	}
	for i := n; i < len(h.entries); i++ {
		h.entries[i].cleanupMedia()
	}
	h.entries = h.entries[:n]
}

// All returns a snapshot of entries, newest first.
func (h *HistoryStore) All() []*Entry {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*Entry, len(h.entries))
	copy(out, h.entries)
	return out
}

// Clear removes all entries.
func (h *HistoryStore) Clear() {
	h.mu.Lock()
	for _, e := range h.entries {
		e.cleanupMedia()
	}
	h.entries = nil
	h.mu.Unlock()
	h.save()
}

// Remove deletes a single entry from the store.
func (h *HistoryStore) Remove(entry *Entry) {
	h.mu.Lock()
	for i, e := range h.entries {
		if e == entry {
			e.cleanupMedia()
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			break
		}
	}
	cb := h.onChanged
	h.mu.Unlock()

	h.save()

	if cb != nil {
		cb()
	}
}

func (h *HistoryStore) save() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	dir := StorageDir()
	path := filepath.Join(dir, "history.json")

	data, err := json.MarshalIndent(h.entries, "", "  ")
	if err != nil {
		log.Printf("error marshaling history: %v", err)
		return
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		log.Printf("error saving history: %v", err)
	}
}

func (h *HistoryStore) Load() {
	h.mu.Lock()
	defer h.mu.Unlock()

	dir := StorageDir()
	path := filepath.Join(dir, "history.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("error reading history file: %v", err)
		}
		return
	}

	var loaded []*Entry
	if err := json.Unmarshal(data, &loaded); err != nil {
		log.Printf("error unmarshaling history: %v", err)
		return
	}

	h.entries = loaded
	go h.GarbageCollectMedia()
}

// GarbageCollectMedia scans the media directory and deletes any files
// that are not referenced in the current history entries.
func (h *HistoryStore) GarbageCollectMedia() {
	h.mu.RLock()
	referenced := make(map[string]bool)
	for _, e := range h.entries {
		e.mu.RLock()
		if e.thumbnailPath != "" {
			referenced[filepath.Clean(e.thumbnailPath)] = true
		}
		e.mu.RUnlock()
	}
	h.mu.RUnlock()

	mDir := mediaDir()
	files, err := os.ReadDir(mDir)
	if err != nil {
		log.Printf("GC: error reading media directory: %v", err)
		return
	}

	count := 0
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		path := filepath.Clean(filepath.Join(mDir, f.Name()))
		if !referenced[path] {
			if err := os.Remove(path); err == nil {
				count++
			} else {
				log.Printf("GC: error removing orphaned media file %s: %v", path, err)
			}
		}
	}
	if count > 0 {
		log.Printf("GC: cleaned up %d orphaned media files from %s", count, mDir)
	}
}
