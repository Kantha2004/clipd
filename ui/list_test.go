package ui

import (
	"fmt"
	"testing"
	"time"

	"clipboard/core"

	"fyne.io/fyne/v2/test"
)

// TestHistoryRowReuse guards the widget.List virtualization wiring added to
// fix the picker being slow to render with a full history: a pooled
// historyRow must rebind to whichever filtered index it's given, and must
// not panic when reused across indices or asked to render an out-of-range id.
func TestHistoryRowReuse(t *testing.T) {
	store := core.NewHistoryStore(24*time.Hour, 200)
	for i := range 50 {
		store.AddText(fmt.Sprintf("entry %d", i))
	}

	u := &UI{app: test.NewApp(), store: store, cursorIdx: -1}
	u.buildWindow()
	u.filtered = store.All()

	if got := u.listLength(); got != len(u.filtered) {
		t.Fatalf("listLength() = %d, want %d", got, len(u.filtered))
	}

	row, ok := u.createHistoryRow().(*historyRow)
	if !ok {
		t.Fatalf("createHistoryRow() did not return a *historyRow")
	}

	// Same pooled row rebinds across two different indices, like a real
	// scroll reusing widgets instead of allocating new ones.
	u.updateHistoryRow(0, row)
	if row.entry != u.filtered[0] {
		t.Fatalf("row.entry after binding id 0 = %v, want %v", row.entry, u.filtered[0])
	}
	u.updateHistoryRow(10, row)
	if row.entry != u.filtered[10] {
		t.Fatalf("row.entry after rebinding id 10 = %v, want %v", row.entry, u.filtered[10])
	}

	// Out-of-range id (can happen transiently while the store shrinks) must
	// not panic and must leave the row's prior binding untouched.
	u.updateHistoryRow(len(u.filtered)+5, row)
	if row.entry != u.filtered[10] {
		t.Fatalf("out-of-range update should not rebind row, got %v", row.entry)
	}
}
