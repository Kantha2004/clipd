package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"golang.design/x/clipboard"
)

// navEntry is a search box that routes ↑ ↓ Enter to the list while still
// passing all other keys to the underlying text entry.
type navEntry struct {
	widget.Entry
	onUp    func()
	onDown  func()
	onEnter func()
}

func newNavEntry() *navEntry {
	e := &navEntry{}
	e.ExtendBaseWidget(e)
	return e
}

func (e *navEntry) TypedKey(key *fyne.KeyEvent) {
	switch key.Name {
	case fyne.KeyUp:
		if e.onUp != nil {
			e.onUp()
		}
	case fyne.KeyDown:
		if e.onDown != nil {
			e.onDown()
		}
	case fyne.KeyReturn, fyne.KeyEnter:
		if e.onEnter != nil {
			e.onEnter()
		}
	default:
		e.Entry.TypedKey(key)
	}
}

// UI owns the picker window and the system tray entry.
type UI struct {
	app    fyne.App
	store  *HistoryStore
	window fyne.Window

	search *navEntry
	list   *widget.List

	filtered   []Entry
	cursorIdx  int  // highlighted row index; -1 = none
	navigating bool // true while programmatically calling list.Select (suppresses action)

	// showCh receives trigger events from the hotkey goroutine.
	showCh chan string
}

func NewUI(a fyne.App, store *HistoryStore) *UI {
	u := &UI{app: a, store: store, showCh: make(chan string, 4), cursorIdx: -1}
	u.buildWindow()
	u.setupSystray()
	return u
}

func (u *UI) buildWindow() {
	u.window = u.app.NewWindow("Clipboard History")
	u.window.Resize(fyne.NewSize(620, 440))
	u.window.CenterOnScreen()
	u.window.SetCloseIntercept(u.hidePicker)

	u.search = newNavEntry()
	u.search.SetPlaceHolder("Type to filter… (↑↓ navigate, Enter select, Esc close)")
	u.search.OnChanged = func(q string) { u.applyFilter(q) }
	u.search.onDown = func() { u.moveCursor(+1) }
	u.search.onUp = func() { u.moveCursor(-1) }
	u.search.onEnter = u.confirmCursor

	u.list = widget.NewList(
		func() int { return len(u.filtered) },
		func() fyne.CanvasObject {
			preview := widget.NewLabel("")
			preview.Wrapping = fyne.TextTruncate
			ts := widget.NewLabel("")
			ts.Importance = widget.LowImportance
			ts.TextStyle = fyne.TextStyle{Italic: true}
			return container.NewVBox(preview, ts)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(u.filtered) {
				return
			}
			e := u.filtered[id]
			c := obj.(*fyne.Container)
			c.Objects[0].(*widget.Label).SetText(e.Preview())
			c.Objects[1].(*widget.Label).SetText(formatAge(e.Timestamp))
		},
	)
	// Mouse click: select immediately.
	u.list.OnSelected = func(id widget.ListItemID) {
		if u.navigating || id >= len(u.filtered) {
			return
		}
		u.selectEntry(u.filtered[id])
	}

	u.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if ev.Name == fyne.KeyEscape {
			u.hidePicker()
		}
	})

	u.window.SetContent(container.NewBorder(
		container.NewVBox(u.search, widget.NewSeparator()),
		nil, nil, nil,
		u.list,
	))
}

// moveCursor moves the highlighted row by delta (+1 down, -1 up).
func (u *UI) moveCursor(delta int) {
	if len(u.filtered) == 0 {
		return
	}
	next := u.cursorIdx + delta
	if next < 0 {
		next = 0
	}
	if next >= len(u.filtered) {
		next = len(u.filtered) - 1
	}
	u.cursorIdx = next
	u.navigating = true
	u.list.Select(u.cursorIdx)
	u.navigating = false
	u.list.ScrollTo(u.cursorIdx)
}

// confirmCursor selects the currently highlighted row (Enter key).
func (u *UI) confirmCursor() {
	if u.cursorIdx >= 0 && u.cursorIdx < len(u.filtered) {
		u.selectEntry(u.filtered[u.cursorIdx])
	}
}

func (u *UI) setupSystray() {
	// Keepalive window: fyne exits when all windows are closed.
	// On GNOME Wayland the systray is unreliable, so we always keep a hidden
	// window open. Closing it only hides it; Quit from the menu truly exits.
	keep := u.app.NewWindow("Clipboard Manager")
	keep.SetContent(widget.NewLabel("Clipboard Manager is running.\nUse your bound key to open history."))
	keep.Resize(fyne.NewSize(320, 80))
	keep.SetCloseIntercept(func() { keep.Hide() })
	keep.Show()

	if desk, ok := u.app.(desktop.App); ok {
		desk.SetSystemTrayMenu(fyne.NewMenu("Clipboard Manager",
			fyne.NewMenuItem("Open History", func() { u.ShowPicker("") }),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Clear History", u.store.Clear),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Quit", u.app.Quit),
		))
	}
}

// ShowPicker is safe to call from any goroutine. It enqueues the request so
// the fyne main loop shows the window on the correct thread.
func (u *UI) ShowPicker(prevWindowID string) {
	log.Printf("ShowPicker queued prevWin=%q", prevWindowID)
	select {
	case u.showCh <- prevWindowID:
	default:
	}
}

// showPickerNow must only be called from the fyne main goroutine.
func (u *UI) showPickerNow(_ string) {
	log.Println("showing picker window")
	u.cursorIdx = -1
	u.search.SetText("")
	u.applyFilter("")
	u.window.Show()
	u.window.RequestFocus()
	u.window.Canvas().Focus(u.search)
}

func (u *UI) hidePicker() {
	u.window.Hide()
}

func (u *UI) applyFilter(query string) {
	all := u.store.All()
	if query == "" {
		u.filtered = all
	} else {
		q := strings.ToLower(query)
		result := make([]Entry, 0, len(all))
		for _, e := range all {
			if strings.Contains(strings.ToLower(e.Content), q) {
				result = append(result, e)
			}
		}
		u.filtered = result
	}
	// Reset cursor to first item so Enter immediately works.
	u.cursorIdx = -1
	u.list.UnselectAll()
	u.list.Refresh()
}

func (u *UI) selectEntry(e Entry) {
	u.hidePicker()
	go u.simulatePaste(e.Content)
}

// simulatePaste writes content to the Wayland clipboard, then tries to send
// Ctrl+V via ydotool (uinput-level, works on GNOME Wayland).
// If ydotool is unavailable it notifies the user to press Ctrl+V manually.
//
// To enable full auto-paste on GNOME Wayland:
//
//	sudo apt install ydotool
//	sudo systemctl enable --now ydotoold
//	sudo usermod -aG input $USER   # then log out and back in
func (u *UI) simulatePaste(content string) {
	// Write to Wayland clipboard.
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(content)
	if err := cmd.Run(); err != nil {
		log.Printf("paste: wl-copy failed (%v), falling back to X11 clipboard", err)
		clipboard.Write(clipboard.FmtText, []byte(content))
	}

	// Wait for compositor to return focus to the previous window.
	time.Sleep(300 * time.Millisecond)

	// Try ydotool first (uinput-level, works everywhere on Wayland).
	if out, err := exec.Command("ydotool", "key", "ctrl+v").CombinedOutput(); err == nil {
		log.Println("paste: Ctrl+V sent via ydotool")
		return
	} else {
		log.Printf("paste: ydotool unavailable (%v: %s)", err, strings.TrimSpace(string(out)))
	}

	// Fall back to xdotool (works for XWayland apps on GNOME Wayland).
	if out, err := exec.Command("xdotool", "key", "ctrl+v").CombinedOutput(); err == nil {
		log.Println("paste: Ctrl+V sent via xdotool")
		return
	} else {
		log.Printf("paste: xdotool failed (%v: %s)", err, strings.TrimSpace(string(out)))
	}

	// Fall back to wtype (works on wlroots compositors like Sway).
	if out, err := exec.Command("wtype", "-M", "ctrl", "-k", "v", "-m", "ctrl").CombinedOutput(); err == nil {
		log.Println("paste: Ctrl+V sent via wtype")
		return
	} else {
		log.Printf("paste: wtype failed (%v: %s)", err, strings.TrimSpace(string(out)))
	}

	// No auto-paste available — notify the user.
	exec.Command("notify-send", "-t", "2000", "-i", "edit-paste", "Clipboard Manager", "Copied — press Ctrl+V to paste").Run()
	log.Println("paste: item in clipboard, press Ctrl+V to paste")
}

// Run starts the fyne event loop (blocks until the app quits).
// It also drains the showCh channel so window operations happen on the main thread.
func (u *UI) Run() {
	// Poll showCh every 50 ms from a goroutine, but call window operations
	// via widget.NewLabel trick to hop onto fyne's internal main thread.
	// On Linux/GLFW, window.Show() is internally dispatched to the main
	// thread, so calling it from this goroutine is safe.
	go func() {
		for prevWin := range u.showCh {
			u.showPickerNow(prevWin)
		}
	}()
	u.app.Run()
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("Jan 2")
	}
}
