package ui

import (
	"bytes"
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"clipboard/config"
	"clipboard/core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.design/x/clipboard"
)

type HistoryStore = core.HistoryStore
type Entry = core.Entry

var shellCommands = map[string]bool{
	"sudo": true, "git": true, "npm": true, "yarn": true, "docker": true,
	"pip": true, "apt": true, "cargo": true, "go": true, "python": true,
	"python3": true, "bash": true, "sh": true, "curl": true, "wget": true,
	"ssh": true, "systemctl": true, "pkill": true, "grep": true, "make": true,
}

const (
	systrayWindowTitle = "Clipboard Manager"
	wlCopyCmd          = "wl-copy"
	genericFileIcon    = "text-x-generic"
)

// UI owns the picker window and the system tray entry.
type UI struct {
	app    fyne.App
	store  *HistoryStore
	window fyne.Window

	search *navEntry
	list   *widget.List

	filtered  []*Entry
	cursorIdx int // highlighted row index; -1 = none

	// showCh receives trigger events from the hotkey goroutine.
	showCh    chan string
	prevWinID string // window that was active when the picker was opened
	visible   bool
}

func NewUI(a fyne.App, store *HistoryStore) *UI {
	u := &UI{app: a, store: store, showCh: make(chan string, 4), cursorIdx: -1}
	u.buildWindow()
	u.setupSystray()

	u.store.SetOnChanged(func() {
		fyne.Do(func() {
			if u.visible {
				u.applyFilter(u.search.Text)
			}
		})
	})

	return u
}

// Globals colorSelected, colorHover, and colorDefault are declared in config.go

func (u *UI) buildWindow() {
	u.window = u.app.NewWindow("Clipd Picker")

	wWidth := config.CurrentTheme.GetSize("window_width")
	wHeight := config.CurrentTheme.GetSize("window_height")
	u.window.Resize(fyne.NewSize(wWidth, wHeight))
	u.window.CenterOnScreen()
	u.window.SetCloseIntercept(u.hidePicker)

	// Header: 📋 Clipd Picker               200 entries max
	headerLabel := &widget.Label{Text: "📋 Clipd Picker", TextStyle: fyne.TextStyle{Bold: true}}
	limitLabel := &widget.Label{Text: "200 entries max", Importance: widget.LowImportance}
	header := container.NewBorder(nil, nil, headerLabel, limitLabel)

	u.search = newNavEntry()
	u.search.SetPlaceHolder("Type to filter history...")
	u.search.OnChanged = func(q string) { u.applyFilter(q) }
	u.search.onDown = func() { u.moveCursor(+1) }
	u.search.onUp = func() { u.moveCursor(-1) }
	u.search.onEnter = u.confirmCursor
	u.search.onShiftEnter = u.confirmCursorShift
	u.search.onEscape = u.hidePicker

	// Esc to close badge inside search row
	escKeyBg := canvas.NewRectangle(config.CurrentTheme.GetColor("button"))
	escKeyBg.StrokeColor = config.CurrentTheme.GetColor("keycap_border")
	escKeyBg.StrokeWidth = 1
	escKeyBg.CornerRadius = config.CurrentTheme.GetSize("keycap_radius")

	escKeyText := canvas.NewText("Esc", config.CurrentTheme.GetColor("foreground"))
	escKeyText.TextStyle = fyne.TextStyle{Bold: true}
	escKeyText.TextSize = 10
	escKeyText.Alignment = fyne.TextAlignCenter

	escKeyContainer := container.New(&badgeLayout{}, escKeyBg, escKeyText)

	toCloseTextObj := canvas.NewText("to close", config.CurrentTheme.GetColor("placeholder"))
	toCloseTextObj.TextSize = 10

	escBadge := container.NewHBox(container.NewCenter(escKeyContainer), container.NewCenter(toCloseTextObj))
	escBadgeContainer := container.NewCenter(escBadge)

	rightSearchSpacer := canvas.NewRectangle(color.Transparent)
	rightSearchSpacer.SetMinSize(fyne.NewSize(8, 0))
	rightSearchSide := container.NewHBox(escBadgeContainer, rightSearchSpacer)

	searchContainer := container.NewStack(
		u.search,
		container.NewBorder(nil, nil, nil, rightSearchSide),
	)

	u.list = widget.NewList(u.listLength, u.createHistoryRow, u.updateHistoryRow)

	// Footer: ↑ ↓ Navigation | Enter Paste | Shift+Enter Term Paste
	keyUp := container.NewCenter(createKeycap("↑"))
	keyDown := container.NewCenter(createKeycap("↓"))
	navText := &widget.Label{Text: "Navigation", Importance: widget.LowImportance}

	divider1 := &widget.Label{Text: "|", Importance: widget.LowImportance}

	keyEnter := container.NewCenter(createKeycap("Enter"))
	pasteText := &widget.Label{Text: "Paste", Importance: widget.LowImportance}

	divider2 := &widget.Label{Text: "|", Importance: widget.LowImportance}

	keyShiftEnter := container.NewCenter(createKeycap("Shift+Enter"))
	termPasteText := &widget.Label{Text: "Term Paste", Importance: widget.LowImportance}

	footer := container.NewHBox(
		layout.NewSpacer(),
		keyUp,
		keyDown,
		navText,
		divider1,
		keyEnter,
		pasteText,
		divider2,
		keyShiftEnter,
		termPasteText,
		layout.NewSpacer(),
	)

	u.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if u.window.Canvas().Focused() == u.search {
			return
		}
		switch ev.Name {
		case fyne.KeyUp:
			u.moveCursor(-1)
		case fyne.KeyDown:
			u.moveCursor(+1)
		case fyne.KeyReturn, fyne.KeyEnter:
			shiftHeld := false
			if drv, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
				if drv.CurrentKeyModifiers()&fyne.KeyModifierShift != 0 {
					shiftHeld = true
				}
			}
			if shiftHeld {
				u.confirmCursorShift()
			} else {
				u.confirmCursor()
			}
		case fyne.KeyEscape:
			u.hidePicker()
		}
	})

	u.window.SetContent(container.NewPadded(container.NewBorder(
		container.NewVBox(header, searchContainer),
		footer, nil, nil,
		u.list,
	)))
}

// historyRow is a pooled, reusable row widget for u.list. Fyne's widget.List
// keeps only as many of these alive as there are visible rows, and rebinds
// them to a different filtered index as the user scrolls — instead of
// building one full widget tree per history entry up front.
type historyRow struct {
	*tappableContainer
	idx       int
	entry     *Entry
	bg        *canvas.Rectangle
	ind       *indicatorBar
	ts        *widget.Label
	preview   *widget.Label
	badgeWrap *fyne.Container
	thumbWrap *fyne.Container
}

func (u *UI) listLength() int {
	return len(u.filtered)
}

func (u *UI) createHistoryRow() fyne.CanvasObject {
	row := &historyRow{idx: -1}

	row.preview = widget.NewLabel("")
	row.preview.TextStyle = fyne.TextStyle{Monospace: true}

	row.badgeWrap = container.NewHBox()
	row.ts = widget.NewLabel("")
	row.ts.Importance = widget.LowImportance
	subtitleBox := container.NewHBox(row.badgeWrap, row.ts)

	row.bg = canvas.NewRectangle(config.ColorDefault)
	row.ind = newIndicatorBar(color.Transparent)
	row.thumbWrap = container.NewHBox()

	btnSize := config.CurrentTheme.GetSize("copy_btn_size")

	copyIcon := newTappableIcon(theme.ContentCopyIcon(), func() {
		if row.entry != nil {
			u.copyOnly(row.entry)
		}
	})
	copyBtn := container.NewGridWrap(fyne.NewSize(btnSize, btnSize), copyIcon)

	removeIcon := newTappableIcon(theme.CancelIcon(), func() {
		if row.entry != nil {
			u.store.Remove(row.entry)
		}
	})
	removeBtn := container.NewGridWrap(fyne.NewSize(btnSize, btnSize), removeIcon)

	leftSpacer := canvas.NewRectangle(color.Transparent)
	leftSpacer.SetMinSize(fyne.NewSize(8, 0))
	leftSide := container.NewHBox(row.ind, leftSpacer, row.thumbWrap)

	btnSpacer := canvas.NewRectangle(color.Transparent)
	btnSpacer.SetMinSize(fyne.NewSize(6, 0))
	rightSpacer := canvas.NewRectangle(color.Transparent)
	rightSpacer.SetMinSize(fyne.NewSize(8, 0))
	rightSide := container.NewHBox(
		container.NewCenter(copyBtn),
		btnSpacer,
		container.NewCenter(removeBtn),
		rightSpacer,
	)

	textCol := container.NewVBox(row.preview, subtitleBox)
	content := container.NewBorder(nil, nil, leftSide, rightSide, textCol)
	item := container.NewStack(row.bg, content)

	row.tappableContainer = newTappableContainer(item, row.bg, row.ts, func() bool {
		return u.cursorIdx == row.idx
	}, func(shiftHeld bool) {
		if row.entry != nil {
			u.selectEntry(row.entry, shiftHeld)
		}
	})

	return row
}

func (u *UI) updateHistoryRow(id widget.ListItemID, obj fyne.CanvasObject) {
	row := obj.(*historyRow)
	if id < 0 || id >= len(u.filtered) {
		return
	}
	entry := u.filtered[id]
	row.idx = id
	row.entry = entry

	row.preview.SetText(SingleLinePreview(entry.Preview()))
	row.ts.SetText(FormatAge(entry.Timestamp))

	row.badgeWrap.Objects = nil
	if badge := getEntryBadge(entry); badge != nil {
		row.badgeWrap.Add(container.NewCenter(badge))
		dot := canvas.NewText(" • ", config.CurrentTheme.GetColor("placeholder"))
		dot.TextSize = 10
		row.badgeWrap.Add(container.NewCenter(dot))
	}
	row.badgeWrap.Refresh()

	row.thumbWrap.Objects = nil
	if thumbObj := createThumbnailObject(entry); thumbObj != nil {
		row.thumbWrap.Add(container.NewCenter(thumbObj))
		thumbSpacer := canvas.NewRectangle(color.Transparent)
		thumbSpacer.SetMinSize(fyne.NewSize(8, 0))
		row.thumbWrap.Add(thumbSpacer)
	}
	row.thumbWrap.Refresh()

	if u.cursorIdx == row.idx {
		row.bg.FillColor = config.ColorHover
		row.ts.Importance = widget.MediumImportance
		row.ind.SetColor(theme.Color(theme.ColorNamePrimary))
	} else {
		row.bg.FillColor = config.ColorDefault
		row.ts.Importance = widget.LowImportance
		row.ind.SetColor(color.Transparent)
	}
	row.bg.Refresh()
	row.ts.Refresh()
	row.Refresh()
}

// createThumbnailObject creates a Fyne CanvasObject preview for the entry
// (either a generated thumbnail image, video/image mime icons, or system/file type icons).
func createThumbnailObject(entry *Entry) fyne.CanvasObject {
	thumbPath := entry.GetThumbnailPath()
	if thumbPath != "" {
		if _, err := os.Stat(thumbPath); err == nil {
			img := canvas.NewImageFromFile(thumbPath)
			img.FillMode = canvas.ImageFillContain
			return container.NewGridWrap(fyne.NewSize(64, 45), img)
		}
	}

	if entry.IsVideo {
		icon := widget.NewIcon(theme.MediaVideoIcon())
		return container.NewGridWrap(fyne.NewSize(64, 45), icon)
	}
	if entry.IsImage {
		icon := widget.NewIcon(theme.FileImageIcon())
		return container.NewGridWrap(fyne.NewSize(64, 45), icon)
	}
	if entry.FilePath != "" {
		if paths := entry.FilePaths(); len(paths) > 1 {
			return stackedFileIcons(paths)
		}
		return container.NewGridWrap(fyne.NewSize(64, 45), fileThumbIcon(entry.FilePath))
	}

	return nil
}

// fileThumbIcon returns the best available icon for a single file: its
// system mime icon if one is found, else Fyne's generic file icon.
func fileThumbIcon(path string) fyne.CanvasObject {
	if iconPath := getSystemMimeIcon(filepath.Ext(path)); iconPath != "" {
		img := canvas.NewImageFromFile(iconPath)
		img.FillMode = canvas.ImageFillContain
		return img
	}
	return widget.NewFileIcon(storage.NewFileURI(path))
}

// stackedFileIcons renders up to 3 of the copied files' own icons in a
// staggered stack, so a multi-file entry reads as "a pile of these files"
// rather than a generic folder.
func stackedFileIcons(paths []string) fyne.CanvasObject {
	const iconSize float32 = 42
	const step float32 = 5

	n := min(len(paths), 3)

	stack := container.NewWithoutLayout()
	for i := n - 1; i >= 0; i-- { // back-to-front so paths[0] ends up on top
		icon := fileThumbIcon(paths[i])
		icon.Resize(fyne.NewSize(iconSize, iconSize))
		icon.Move(fyne.NewPos(float32(i)*step, float32(i)*step))
		stack.Add(icon)
	}

	// container.NewWithoutLayout reports MinSize as just its largest child
	// (28x28), ignoring the offset stagger — wrapping it in a GridWrap cell
	// forces the true bounding box (~44x44) so NewCenter positions it
	// correctly instead of letting the back icons spill outside the row.
	total := iconSize + float32(n-1)*step
	fixed := container.NewGridWrap(fyne.NewSize(total, total), stack)

	return container.NewGridWrap(fyne.NewSize(64, 45), container.NewCenter(fixed))
}

// moveCursor moves the highlighted row by delta (+1 down, -1 up).
func (u *UI) moveCursor(delta int) {
	if len(u.filtered) == 0 {
		return
	}
	next := u.cursorIdx + delta
	if next < 0 {
		u.setCursor(-1)
		u.window.Canvas().Focus(u.search)
		return
	}
	if next >= len(u.filtered) {
		next = len(u.filtered) - 1
	}
	u.setCursor(next)
}

func (u *UI) setCursor(idx int) {
	old := u.cursorIdx
	u.cursorIdx = idx

	// Refresh only rebinds visible rows, so this stays cheap regardless of
	// how many entries are in history.
	if old >= 0 && old < len(u.filtered) {
		u.list.RefreshItem(old)
	}
	if idx >= 0 && idx < len(u.filtered) {
		u.list.RefreshItem(idx)
		u.list.ScrollTo(idx) // keeps cursor visible
	}
}

// confirmCursor selects the currently highlighted row (Enter key).
func (u *UI) confirmCursor() {
	if u.cursorIdx >= 0 && u.cursorIdx < len(u.filtered) {
		u.selectEntry(u.filtered[u.cursorIdx], false)
	}
}

// confirmCursorShift selects the currently highlighted row with Shift held (Shift+Enter).
func (u *UI) confirmCursorShift() {
	if u.cursorIdx >= 0 && u.cursorIdx < len(u.filtered) {
		u.selectEntry(u.filtered[u.cursorIdx], true)
	}
}

func (u *UI) setupSystray() {
	// Keepalive window: fyne exits when all windows are closed.
	// On GNOME Wayland the systray is unreliable, so we keep this window
	// registered but hidden by default. Closing it only hides it.
	keep := u.app.NewWindow(systrayWindowTitle)
	keep.SetContent(widget.NewLabel("Clipboard Manager is running.\nUse your bound key to open history."))
	keep.Resize(fyne.NewSize(320, 80))
	keep.SetCloseIntercept(func() { keep.Hide() })

	if desk, ok := u.app.(desktop.App); ok {
		desk.SetSystemTrayIcon(ResourceIconSvg)
		desk.SetSystemTrayMenu(fyne.NewMenu(systrayWindowTitle,
			fyne.NewMenuItem("Open History", func() { u.ShowPicker("") }),
			fyne.NewMenuItem("Show Status", keep.Show),
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
func (u *UI) showPickerNow(prevWinID string) {
	log.Println("showing picker window")
	fyne.Do(func() {
		u.prevWinID = prevWinID
		u.cursorIdx = -1
		u.search.SetText("")
		u.applyFilter("")
		u.visible = true
		u.window.Show()
		u.window.RequestFocus()
		u.window.Canvas().Focus(u.search)
	})
}

func (u *UI) hidePicker() {
	u.visible = false
	u.window.Hide()
}

func (u *UI) applyFilter(query string) {
	all := u.store.All()
	if query == "" {
		u.filtered = all
	} else {
		q := strings.ToLower(query)
		result := make([]*Entry, 0, len(all))
		for _, e := range all {
			if strings.Contains(strings.ToLower(e.Content), q) {
				result = append(result, e)
			}
		}
		u.filtered = result
	}
	u.cursorIdx = -1
	u.list.Refresh()
}

// buildURIList joins file paths into a text/uri-list body (CRLF-separated
// file:// URIs), so copying a multi-file entry pastes back all the files.
func buildURIList(paths []string) string {
	var b strings.Builder
	for _, p := range paths {
		b.WriteString("file://")
		b.WriteString(p)
		b.WriteString("\r\n")
	}
	return b.String()
}

func writeToClipboard(e *Entry) error {
	if e.IsImage && len(e.ImageData) > 0 {
		cmd := exec.Command(wlCopyCmd, "-t", "image/png")
		cmd.Stdin = bytes.NewReader(e.ImageData)
		if err := cmd.Run(); err == nil {
			return nil
		}
		// Fallback to X11 image clipboard
		changed := clipboard.Write(clipboard.FmtImage, e.ImageData)
		go func() { <-changed }()
		return nil
	}

	if e.FilePath != "" {
		paths := e.FilePaths()
		if len(paths) == 0 {
			paths = []string{e.FilePath}
		}
		cmd := exec.Command(wlCopyCmd, "-t", "text/uri-list")
		cmd.Stdin = strings.NewReader(buildURIList(paths))
		if err := cmd.Run(); err == nil {
			return nil
		}
		// Fallback to X11 plain text path
		changed := clipboard.Write(clipboard.FmtText, []byte(e.Content))
		go func() { <-changed }()
		return nil
	}

	cmd := exec.Command(wlCopyCmd)
	cmd.Stdin = strings.NewReader(e.Content)
	if err := cmd.Run(); err != nil {
		changed := clipboard.Write(clipboard.FmtText, []byte(e.Content))
		go func() { <-changed }()
	}
	return nil
}

func (u *UI) selectEntry(e *Entry, shiftHeld bool) {
	prevWinID := u.prevWinID // capture before hiding
	u.hidePicker()
	go u.simulatePaste(e, shiftHeld, prevWinID)
}

// copyOnly writes content to the clipboard without pasting.
func (u *UI) copyOnly(e *Entry) {
	u.hidePicker()
	go func() {
		if err := writeToClipboard(e); err != nil {
			log.Printf("copy failed: %v", err)
		} else {
			log.Println("copy: item copied to clipboard (no paste)")
		}
	}()
}

// simulatePaste writes content to the Wayland clipboard, then tries to send
// Ctrl+V (or Ctrl+Shift+V when shiftHeld is true) via ydotool.
// If ydotool is unavailable it notifies the user to paste manually.
//
// To enable full auto-paste on GNOME Wayland:
//
//	sudo apt install ydotool
//	sudo systemctl enable --now ydotoold
//	sudo usermod -aG input $USER   # then log out and back in
func (u *UI) simulatePaste(e *Entry, shiftHeld bool, prevWinID string) {
	// Write to Wayland clipboard.
	if err := writeToClipboard(e); err != nil {
		log.Printf("paste: copy to clipboard failed: %v", err)
	}

	// Refocus the previous window so the key event lands in the right place.
	if prevWinID != "" {
		exec.Command("xdotool", "windowfocus", "--sync", prevWinID).Run()
		time.Sleep(100 * time.Millisecond)
	} else {
		// No window ID — wait for compositor to return focus on its own.
		time.Sleep(300 * time.Millisecond)
	}

	// Choose the key combo based on whether Shift was held at selection time.
	// Ctrl+Shift+V is used by terminals (e.g. GNOME Terminal, Kitty) for paste.
	ydotoolKey := "ctrl+v"
	xdotoolKey := "ctrl+v"
	wtypeArgs := []string{"-M", "ctrl", "-k", "v", "-m", "ctrl"}
	if shiftHeld {
		ydotoolKey = "ctrl+shift+v"
		xdotoolKey = "ctrl+shift+v"
		wtypeArgs = []string{"-M", "ctrl", "-M", "shift", "-k", "v", "-m", "shift", "-m", "ctrl"}
	}

	// Try ydotool first (uinput-level, works everywhere on Wayland).
	if out, err := exec.Command("ydotool", "key", ydotoolKey).CombinedOutput(); err == nil {
		log.Printf("paste: %s sent via ydotool", ydotoolKey)
		return
	} else {
		log.Printf("paste: ydotool unavailable (%v: %s)", err, strings.TrimSpace(string(out)))
	}

	// Fall back to xdotool (works for XWayland apps on GNOME Wayland).
	if out, err := exec.Command("xdotool", "key", xdotoolKey).CombinedOutput(); err == nil {
		log.Printf("paste: %s sent via xdotool", xdotoolKey)
		return
	} else {
		log.Printf("paste: xdotool failed (%v: %s)", err, strings.TrimSpace(string(out)))
	}

	// Fall back to wtype (works on wlroots compositors like Sway).
	if out, err := exec.Command("wtype", wtypeArgs...).CombinedOutput(); err == nil {
		log.Printf("paste: key sent via wtype (shift=%v)", shiftHeld)
		return
	} else {
		log.Printf("paste: wtype failed (%v: %s)", err, strings.TrimSpace(string(out)))
	}

	// No auto-paste available — notify the user.
	pasteKey := "Ctrl+V"
	if shiftHeld {
		pasteKey = "Ctrl+Shift+V"
	}
	exec.Command("notify-send", "-t", "2000", "-i", "edit-paste", systrayWindowTitle,
		"Copied — press "+pasteKey+" to paste").Run()
	log.Printf("paste: item in clipboard, press %s to paste", pasteKey)
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
	close(u.showCh) // unblock the drain goroutine so it exits cleanly
}

func FormatAge(t time.Time) string {
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

type badgeLayout struct{}

func (b *badgeLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.NewSize(0, 0)
	}
	textSize := objects[1].MinSize()
	padX := config.CurrentTheme.GetSize("keycap_padding_x")
	padY := config.CurrentTheme.GetSize("keycap_padding_y")
	return fyne.NewSize(textSize.Width+padX, textSize.Height+padY)
}

func (b *badgeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	objects[0].Resize(size) // bg
	textSize := objects[1].MinSize()
	objects[1].Resize(textSize)
	yPos := (size.Height-textSize.Height)/2 - 1.0
	objects[1].Move(fyne.NewPos((size.Width-textSize.Width)/2, yPos))
}

func newBadge(text string, bgColor, fgColor color.Color) fyne.CanvasObject {
	bg := canvas.NewRectangle(bgColor)
	bg.CornerRadius = config.CurrentTheme.GetSize("badge_radius")
	txt := canvas.NewText(text, fgColor)
	txt.TextStyle = fyne.TextStyle{Bold: true}
	txt.TextSize = 9
	txt.Alignment = fyne.TextAlignCenter
	return container.New(&badgeLayout{}, bg, txt)
}

func createKeycap(text string) fyne.CanvasObject {
	bg := canvas.NewRectangle(config.CurrentTheme.GetColor("button"))
	bg.StrokeColor = config.CurrentTheme.GetColor("keycap_border")
	bg.StrokeWidth = 1
	bg.CornerRadius = config.CurrentTheme.GetSize("keycap_radius")

	txt := canvas.NewText(text, config.CurrentTheme.GetColor("foreground"))
	txt.TextStyle = fyne.TextStyle{Bold: true}
	txt.TextSize = 9
	txt.Alignment = fyne.TextAlignCenter

	return container.New(&badgeLayout{}, bg, txt)
}

func getEntryBadge(entry *Entry) fyne.CanvasObject {
	if entry.IsImage {
		if entry.FilePath != "" {
			return newBadge("IMAGE FILE", config.CurrentTheme.GetColor("badge_image_bg"), config.CurrentTheme.GetColor("badge_image_fg"))
		}
		return newBadge("IMAGE", config.CurrentTheme.GetColor("badge_image_bg"), config.CurrentTheme.GetColor("badge_image_fg"))
	}
	if entry.IsVideo {
		return newBadge("VIDEO", config.CurrentTheme.GetColor("badge_video_bg"), config.CurrentTheme.GetColor("badge_video_fg"))
	}
	if entry.FilePath != "" {
		if paths := entry.FilePaths(); len(paths) > 1 {
			return newBadge(fmt.Sprintf("%d FILES", len(paths)), config.CurrentTheme.GetColor("badge_file_bg"), config.CurrentTheme.GetColor("badge_file_fg"))
		}
		ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(entry.FilePath), "."))
		if ext == "" {
			ext = "FILE"
		}
		return newBadge(ext, config.CurrentTheme.GetColor("badge_file_bg"), config.CurrentTheme.GetColor("badge_file_fg"))
	}

	contentTrim := strings.TrimSpace(entry.Content)
	if contentTrim == "" {
		return nil
	}
	// Check if it's a URL
	if strings.HasPrefix(contentTrim, "http://") ||
		strings.HasPrefix(contentTrim, "https://") ||
		strings.HasPrefix(contentTrim, "ftp://") ||
		strings.HasPrefix(contentTrim, "git@github.com:") {
		return newBadge("LINK", config.CurrentTheme.GetColor("badge_link_bg"), config.CurrentTheme.GetColor("badge_link_fg"))
	}

	firstWord, _, _ := strings.Cut(contentTrim, " ")
	isCmd := shellCommands[firstWord]
	if !isCmd && (strings.HasPrefix(contentTrim, "./") || strings.HasPrefix(contentTrim, "source ") || strings.Contains(contentTrim, " && ") || strings.Contains(contentTrim, " || ") || strings.Contains(contentTrim, " | ")) {
		isCmd = true
	}

	if isCmd {
		return newBadge("CMD", config.CurrentTheme.GetColor("badge_cmd_bg"), config.CurrentTheme.GetColor("badge_cmd_fg"))
	}

	return nil
}

func SingleLinePreview(content string) string {
	contentTrim := strings.TrimSpace(content)
	if contentTrim == "" {
		return ""
	}
	firstLine, _, _ := strings.Cut(contentTrim, "\n")
	firstLine = strings.TrimSpace(firstLine)
	runes := []rune(firstLine)
	if len(runes) > 50 {
		return string(runes[:50]) + "..."
	}
	return firstLine
}

var (
	mimeIconCacheMu sync.RWMutex
	mimeIconCache   = map[string]string{}
)

// getSystemMimeIcon is called on every render of every file-backed row, but
// the on-disk icon theme doesn't change at runtime, so cache the lookup per extension.
func getSystemMimeIcon(ext string) string {
	ext = strings.ToLower(ext)

	mimeIconCacheMu.RLock()
	cached, ok := mimeIconCache[ext]
	mimeIconCacheMu.RUnlock()
	if ok {
		return cached
	}

	path := lookupSystemMimeIcon(ext)

	mimeIconCacheMu.Lock()
	mimeIconCache[ext] = path
	mimeIconCacheMu.Unlock()

	return path
}

func lookupSystemMimeIcon(ext string) string {
	var iconNames []string
	switch ext {
	case ".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".rar", ".7z", ".pkg":
		iconNames = []string{"application-x-zip", "application-archive-zip", "package-x-generic"}
	case ".pdf":
		iconNames = []string{"application-pdf"}
	case ".deb":
		iconNames = []string{"application-vnd.debian.binary-package", "application-x-deb", "package-x-generic"}
	case ".mp3", ".wav", ".ogg", ".flac", ".m4a", ".aac":
		iconNames = []string{"audio-x-generic", "audio-mpeg", "audio-x-wav"}
	case ".py":
		iconNames = []string{"text-x-python", "text-x-python3"}
	case ".sh", ".bash", ".zsh", ".run":
		iconNames = []string{"application-x-shellscript", "text-x-script", "application-x-executable"}
	case ".go":
		iconNames = []string{"text-x-go"}
	case ".js", ".jsx":
		iconNames = []string{"application-javascript", "text-x-javascript"}
	case ".ts", ".tsx":
		iconNames = []string{"application-typescript", "application-x-typescript"}
	case ".txt", ".log", ".ini", ".conf":
		iconNames = []string{"text-x-plain", genericFileIcon, "text"}
	case ".md":
		iconNames = []string{"text-markdown", genericFileIcon}
	case ".doc", ".docx", ".odt":
		iconNames = []string{"application-vnd.openxmlformats-officedocument.wordprocessingml.document", "x-office-document", genericFileIcon}
	case ".xls", ".xlsx", ".ods", ".csv":
		iconNames = []string{"application-vnd.openxmlformats-officedocument.spreadsheetml.sheet", "x-office-spreadsheet"}
	case ".ppt", ".pptx", ".odp":
		iconNames = []string{"application-vnd.openxmlformats-officedocument.presentationml.presentation", "x-office-presentation"}
	}

	searchDirs := []struct{ dir, suffix string }{
		{"/usr/share/icons/Yaru/256x256/mimetypes", ".png"},
		{"/usr/share/icons/Yaru/scalable/mimetypes", "-symbolic.svg"},
		{"/usr/share/icons/hicolor/256x256/mimetypes", ".png"},
	}
	for _, d := range searchDirs {
		for _, name := range iconNames {
			if p := filepath.Join(d.dir, name+d.suffix); fileExists(p) {
				return p
			}
		}
	}

	// Generic file icon fallback
	for _, d := range searchDirs[:2] {
		if p := filepath.Join(d.dir, genericFileIcon+d.suffix); fileExists(p) {
			return p
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
