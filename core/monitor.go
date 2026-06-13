package core

import (
	"context"
	"fmt"
	"log"

	"golang.design/x/clipboard"
)

// RunMonitor watches both system clipboard formats (text and images) and feeds updates to HistoryStore.
func RunMonitor(ctx context.Context, store *HistoryStore) error {
	if err := clipboard.Init(); err != nil {
		return fmt.Errorf("clipboard init: %w", err)
	}

	log.Println("clipboard monitor started")

	go func() {
		for data := range clipboard.Watch(ctx, clipboard.FmtText) {
			store.AddText(string(data))
		}
	}()

	go func() {
		for data := range clipboard.Watch(ctx, clipboard.FmtImage) {
			store.AddImage(data)
		}
	}()

	<-ctx.Done()
	return nil
}
