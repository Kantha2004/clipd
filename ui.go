package main

import (
	"fmt"
	"image/color"
	"log"
	"os/exec"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.design/x/clipboard"
)

// navEntry is a search box that routes ↑ ↓ Enter to the list while still
// passing all other keys to the underlying text entry.
type navEntry struct {
	widget.Entry
	onUp     func()
	onDown   func()
	onEnter  func()
	onEscape func()
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
	case fyne.KeyEscape:
		if e.onEscape != nil {
			e.onEscape()
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

	search   *navEntry
	itemsBox *fyne.Container
	scroll   *container.Scroll

	filtered  []Entry
	cursorIdx int // highlighted row index; -1 = none
	itemBgs   []*canvas.Rectangle
	itemTs    []*widget.Label

	// showCh receives trigger events from the hotkey goroutine.
	showCh chan string
}

func NewUI(a fyne.App, store *HistoryStore) *UI {
	u := &UI{app: a, store: store, showCh: make(chan string, 4), cursorIdx: -1}
	u.buildWindow()
	u.setupSystray()
	return u
}

var (
	colorSelected = color.NRGBA{R: 65, G: 105, B: 225, A: 80}
	colorDefault  = color.NRGBA{R: 0, G: 0, B: 0, A: 0}
)

func (u *UI) buildWindow() {
	u.window = u.app.NewWindow("Clipboard History")
	u.window.Resize(fyne.NewSize(620, 440))
	u.window.CenterOnScreen()
	u.window.SetCloseIntercept(u.hidePicker)

	u.search = newNavEntry()
	u.search.SetPlaceHolder("Type to filter... (↑↓ navigate, Enter select, Esc close)")
	u.search.OnChanged = func(q string) { u.applyFilter(q) }
	u.search.onDown = func() { u.moveCursor(+1) }
	u.search.onUp = func() { u.moveCursor(-1) }
	u.search.onEnter = u.confirmCursor
	u.search.onEscape = u.hidePicker

	u.itemsBox = container.NewVBox()
	u.scroll = container.NewVScroll(u.itemsBox)

	u.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if ev.Name == fyne.KeyEscape {
			u.hidePicker()
		}
	})

	u.window.SetContent(container.NewBorder(
		container.NewVBox(u.search, widget.NewSeparator()),
		nil, nil, nil,
		u.scroll,
	))
}

func (u *UI) buildItems() {
	u.itemsBox.Objects = nil
	u.itemBgs = nil
	u.itemTs = nil

	for i, e := range u.filtered {
		entry := e
		_ = i

		preview := widget.NewLabel(entry.Preview())
		ts := widget.NewLabel(formatAge(entry.Timestamp))
		ts.Importance = widget.LowImportance
		ts.TextStyle = fyne.TextStyle{Italic: true}

		copyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
			u.copyOnly(entry)
		})
		copyBtn.Importance = widget.LowImportance

		bg := canvas.NewRectangle(colorDefault)
		u.itemBgs = append(u.itemBgs, bg)
		u.itemTs = append(u.itemTs, ts)

		textCol := container.NewVBox(preview, ts)
		content := container.NewBorder(nil, nil, copyBtn, nil, textCol)
		item := container.NewStack(bg, content)

		tappable := newTappableContainer(item, bg, ts, func() {
			u.selectEntry(entry)
		})

		u.itemsBox.Add(tappable)
		u.itemsBox.Add(widget.NewSeparator())
	}
	u.itemsBox.Refresh()
}

var colorHover = color.NRGBA{R: 255, G: 255, B: 255, A: 30}

// tappableContainer wraps any CanvasObject to make it clickable and hoverable.
type tappableContainer struct {
	widget.BaseWidget
	content  fyne.CanvasObject
	bg       *canvas.Rectangle
	ts       *widget.Label
	onTapped func()
}

func newTappableContainer(content fyne.CanvasObject, bg *canvas.Rectangle, ts *widget.Label, onTapped func()) *tappableContainer {
	t := &tappableContainer{content: content, bg: bg, ts: ts, onTapped: onTapped}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableContainer) CreateRenderer() fyne.WidgetRenderer {
	return &tappableRenderer{content: t.content}
}

func (t *tappableContainer) Tapped(_ *fyne.PointEvent) {
	if t.onTapped != nil {
		t.onTapped()
	}
}

func (t *tappableContainer) MouseIn(_ *desktop.MouseEvent) {
	if t.bg.FillColor != colorSelected {
		t.bg.FillColor = colorHover
		t.bg.Refresh()
		t.ts.Importance = widget.MediumImportance
		t.ts.Refresh()
	}
}

func (t *tappableContainer) MouseMoved(_ *desktop.MouseEvent) {}

func (t *tappableContainer) MouseOut() {
	if t.bg.FillColor != colorSelected {
		t.bg.FillColor = colorDefault
		t.bg.Refresh()
		t.ts.Importance = widget.LowImportance
		t.ts.Refresh()
	}
}

type tappableRenderer struct {
	content fyne.CanvasObject
}

func (r *tappableRenderer) Layout(size fyne.Size) {
	r.content.Resize(size)
}

func (r *tappableRenderer) MinSize() fyne.Size {
	return r.content.MinSize()
}

func (r *tappableRenderer) Refresh() {
	r.content.Refresh()
}

func (r *tappableRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}

func (r *tappableRenderer) Destroy() {}

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
	u.setCursor(next)
}

func (u *UI) setCursor(idx int) {
	// Clear previous highlight.
	if u.cursorIdx >= 0 && u.cursorIdx < len(u.itemBgs) {
		u.itemBgs[u.cursorIdx].FillColor = colorDefault
		u.itemBgs[u.cursorIdx].Refresh()
		u.itemTs[u.cursorIdx].Importance = widget.LowImportance
		u.itemTs[u.cursorIdx].Refresh()
	}
	u.cursorIdx = idx
	// Set new highlight.
	if u.cursorIdx >= 0 && u.cursorIdx < len(u.itemBgs) {
		u.itemBgs[u.cursorIdx].FillColor = colorSelected
		u.itemBgs[u.cursorIdx].Refresh()
		u.itemTs[u.cursorIdx].Importance = widget.MediumImportance
		u.itemTs[u.cursorIdx].Refresh()

		// Scroll to keep cursor visible. Each item is at index idx*2 (item + separator).
		obj := u.itemsBox.Objects[idx*2]
		pos := obj.Position()
		u.scroll.Offset = fyne.NewPos(0, pos.Y)
		u.scroll.Refresh()
	}
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

// showPickerNow schedules the picker to open on the Fyne main thread.
func (u *UI) showPickerNow(_ string) {
	log.Println("showing picker window")
	fyne.Do(func() {
		u.cursorIdx = -1
		u.search.SetText("")
		u.applyFilter("")
		u.window.Show()
		u.window.RequestFocus()
		u.window.Canvas().Focus(u.search)
	})
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
	u.cursorIdx = -1
	u.buildItems()
}

func (u *UI) selectEntry(e Entry) {
	u.hidePicker()
	go u.simulatePaste(e.Content)
}

// copyOnly writes content to the clipboard without pasting.
func (u *UI) copyOnly(e Entry) {
	u.hidePicker()
	go func() {
		cmd := exec.Command("wl-copy")
		cmd.Stdin = strings.NewReader(e.Content)
		if err := cmd.Run(); err != nil {
			log.Printf("copy: wl-copy failed (%v), falling back to X11 clipboard", err)
			clipboard.Write(clipboard.FmtText, []byte(e.Content))
		}
		log.Println("copy: item copied to clipboard (no paste)")
	}()
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
	// Send both Ctrl+Shift+V (terminals) and Ctrl+V (other apps).
	// Non-terminal apps ignore Ctrl+Shift+V; terminals ignore Ctrl+V.
	if out, err := exec.Command("ydotool", "key", "ctrl+shift+v").CombinedOutput(); err == nil {
		time.Sleep(50 * time.Millisecond)
		exec.Command("ydotool", "key", "ctrl+v").Run()
		log.Println("paste: Ctrl+Shift+V and Ctrl+V sent via ydotool")
		return
	} else {
		log.Printf("paste: ydotool unavailable (%v: %s)", err, strings.TrimSpace(string(out)))
	}

	// Fall back to xdotool (works for XWayland apps on GNOME Wayland).
	if out, err := exec.Command("xdotool", "key", "ctrl+shift+v").CombinedOutput(); err == nil {
		time.Sleep(50 * time.Millisecond)
		exec.Command("xdotool", "key", "ctrl+v").Run()
		log.Println("paste: Ctrl+Shift+V and Ctrl+V sent via xdotool")
		return
	} else {
		log.Printf("paste: xdotool failed (%v: %s)", err, strings.TrimSpace(string(out)))
	}

	// Fall back to wtype (works on wlroots compositors like Sway).
	if out, err := exec.Command("wtype", "-M", "ctrl", "-M", "shift", "-k", "v", "-m", "shift", "-m", "ctrl").CombinedOutput(); err == nil {
		time.Sleep(50 * time.Millisecond)
		exec.Command("wtype", "-M", "ctrl", "-k", "v", "-m", "ctrl").Run()
		log.Println("paste: Ctrl+Shift+V and Ctrl+V sent via wtype")
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
