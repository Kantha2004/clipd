package core_test

import (
	"crypto/rand"
	"fmt"
	"runtime"
	"testing"
	"time"

	"clipboard/core"
)

// TestPerformance measures the memory allocations, CPU latency, and throughput
// of HistoryStore under active high-frequency write operations.
func TestPerformance(t *testing.T) {
	// Initialize HistoryStore with standard settings (24h limit, 200 items max)
	store := core.NewHistoryStore(24*time.Hour, 200)

	// Measure initial memory baseline
	var msStart runtime.MemStats
	runtime.ReadMemStats(&msStart)
	t.Logf("[STATS] Initial Memory: Alloc = %.2f MB, Sys = %.2f MB, HeapObjects = %d",
		float64(msStart.Alloc)/1024/1024,
		float64(msStart.Sys)/1024/1024,
		msStart.HeapObjects)

	// 1. Text Copy Performance Load: 1000 high-frequency operations
	t.Log("[LOAD] Injecting 1000 text copy operations...")
	startCopy := time.Now()
	for i := 0; i < 1000; i++ {
		content := fmt.Sprintf("Performance load stress test text item %d - timestamp %v", i, time.Now().UnixNano())
		store.AddText(content)
	}
	elapsedCopy := time.Since(startCopy)
	t.Logf("[STATS] 1000 text copies finished in %v (avg %.3f ms/copy)",
		elapsedCopy,
		float64(elapsedCopy.Nanoseconds())/1000000/1000)

	// 2. Raw Image Copy Performance Load: 50 operations of 100KB images
	t.Log("[LOAD] Injecting 50 raw image copies (100KB each)...")
	startImg := time.Now()
	imgData := make([]byte, 100*1024) // 100 KB image data
	_, _ = rand.Read(imgData)

	for i := 0; i < 50; i++ {
		// Alter the first byte to prevent identical-image deduplication logic
		imgData[0] = byte(i)
		store.AddImage(imgData)
	}
	elapsedImg := time.Since(startImg)
	t.Logf("[STATS] 50 image copies finished in %v (avg %.3f ms/copy)",
		elapsedImg,
		float64(elapsedImg.Nanoseconds())/1000000/50)

	// Force garbage collection to measure memory cleanup recovery
	runtime.GC()

	// Measure final memory footprint after cleanup
	var msEnd runtime.MemStats
	runtime.ReadMemStats(&msEnd)
	t.Logf("[STATS] Final Memory (post-GC): Alloc = %.2f MB, Sys = %.2f MB, HeapObjects = %d",
		float64(msEnd.Alloc)/1024/1024,
		float64(msEnd.Sys)/1024/1024,
		msEnd.HeapObjects)

	// Assertions to verify throughput standards
	// 1000 text operations should execute within 1.5 seconds
	if elapsedCopy > 1500*time.Millisecond {
		t.Errorf("Performance SLA Violated: 1000 text copies took too long (%v)", elapsedCopy)
	}
	// 50 image write operations should execute within 1.0 second
	if elapsedImg > 1000*time.Millisecond {
		t.Errorf("Performance SLA Violated: 50 image copies took too long (%v)", elapsedImg)
	}
}
