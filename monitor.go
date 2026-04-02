package main

import (
	"context"
	"fmt"
	"log"

	"golang.design/x/clipboard"
)

// RunMonitor watches the system clipboard and feeds text changes into store.
// Requires X11; on Wayland you may need XWayland running.
func RunMonitor(ctx context.Context, store *HistoryStore) error {
	if err := clipboard.Init(); err != nil {
		return fmt.Errorf("clipboard init: %w", err)
	}

	log.Println("clipboard monitor started")
	ch := clipboard.Watch(ctx, clipboard.FmtText)
	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return nil
			}
			store.Add(string(data))
		case <-ctx.Done():
			return nil
		}
	}
}
