package ui

import (
	"image/color"
	"time"

	"clipboard/config"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// navEntry is a search box that routes ↑ ↓ Enter to the list while still
// passing all other keys to the underlying text entry.
type navEntry struct {
	widget.Entry
	onUp         func()
	onDown       func()
	onEnter      func()
	onShiftEnter func()
	onEscape     func()
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

// TypedShortcut intercepts Shift+Enter to trigger onShiftEnter.
func (e *navEntry) TypedShortcut(s fyne.Shortcut) {
	if cs, ok := s.(*desktop.CustomShortcut); ok {
		if (cs.KeyName == fyne.KeyReturn || cs.KeyName == fyne.KeyEnter) &&
			cs.Modifier == fyne.KeyModifierShift {
			if e.onShiftEnter != nil {
				e.onShiftEnter()
			}
			return
		}
	}
	e.Entry.TypedShortcut(s)
}

// tappableContainer wraps any CanvasObject to make it clickable and hoverable.
type tappableContainer struct {
	widget.BaseWidget
	content    fyne.CanvasObject
	bg         *canvas.Rectangle
	ts         *widget.Label
	isSelected func() bool
	onTapped   func(shiftHeld bool)
}

func newTappableContainer(content fyne.CanvasObject, bg *canvas.Rectangle, ts *widget.Label, isSelected func() bool, onTapped func(shiftHeld bool)) *tappableContainer {
	t := &tappableContainer{content: content, bg: bg, ts: ts, isSelected: isSelected, onTapped: onTapped}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableContainer) CreateRenderer() fyne.WidgetRenderer {
	return &tappableRenderer{content: t.content}
}

func (t *tappableContainer) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button == desktop.MouseButtonPrimary {
		shiftHeld := ev.Modifier&fyne.KeyModifierShift != 0
		if t.onTapped != nil {
			t.onTapped(shiftHeld)
		}
	}
}

func (t *tappableContainer) MouseUp(_ *desktop.MouseEvent) {}

func (t *tappableContainer) MouseIn(_ *desktop.MouseEvent) {
	if t.isSelected == nil || !t.isSelected() {
		t.bg.FillColor = config.ColorHover
		t.bg.Refresh()
		t.ts.Importance = widget.MediumImportance
		t.ts.Refresh()
	}
}

func (t *tappableContainer) MouseMoved(_ *desktop.MouseEvent) {}

func (t *tappableContainer) MouseOut() {
	if t.isSelected == nil || !t.isSelected() {
		t.bg.FillColor = config.ColorDefault
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

// indicatorBar is a custom widget representing a left-side selection bar.
type indicatorBar struct {
	widget.BaseWidget
	rect *canvas.Rectangle
}

func newIndicatorBar(c color.Color) *indicatorBar {
	rect := canvas.NewRectangle(c)
	b := &indicatorBar{rect: rect}
	b.ExtendBaseWidget(b)
	return b
}

func (b *indicatorBar) CreateRenderer() fyne.WidgetRenderer {
	return &indicatorRenderer{rect: b.rect}
}

func (b *indicatorBar) SetColor(c color.Color) {
	b.rect.FillColor = c
	b.rect.Refresh()
}

type indicatorRenderer struct {
	rect *canvas.Rectangle
}

func (r *indicatorRenderer) Layout(size fyne.Size) {
	r.rect.Resize(size)
}

func (r *indicatorRenderer) MinSize() fyne.Size {
	return fyne.NewSize(5, 0)
}

func (r *indicatorRenderer) Refresh() {
	r.rect.Refresh()
}

func (r *indicatorRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.rect}
}

func (r *indicatorRenderer) Destroy() {}

// tappableIcon is an icon that triggers a function when tapped, with hover and click feedback.
type tappableIcon struct {
	widget.BaseWidget
	bg       *canvas.Rectangle
	icon     *widget.Icon
	onTapped func()
	hovered  bool
}

func newTappableIcon(res fyne.Resource, onTapped func()) *tappableIcon {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = config.CurrentTheme.GetSize("copy_btn_radius")

	icon := widget.NewIcon(res)

	t := &tappableIcon{bg: bg, icon: icon, onTapped: onTapped}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableIcon) CreateRenderer() fyne.WidgetRenderer {
	return &tappableIconRenderer{t: t}
}

func (t *tappableIcon) Tapped(ev *fyne.PointEvent) {
	// Visual flash effect on click
	t.bg.FillColor = config.CurrentTheme.GetColor("copy_btn_flash")
	t.bg.Refresh()

	if t.onTapped != nil {
		t.onTapped()
	}

	// Reset back to hover color after a short delay
	go func() {
		time.Sleep(150 * time.Millisecond)
		fyne.Do(func() {
			if t.hovered {
				t.bg.FillColor = config.CurrentTheme.GetColor("copy_btn_hover")
			} else {
				t.bg.FillColor = color.Transparent
			}
			t.bg.Refresh()
		})
	}()
}

func (t *tappableIcon) MouseIn(ev *desktop.MouseEvent) {
	t.hovered = true
	t.bg.FillColor = config.CurrentTheme.GetColor("copy_btn_hover")
	t.bg.Refresh()
}

func (t *tappableIcon) MouseMoved(ev *desktop.MouseEvent) {}

func (t *tappableIcon) MouseOut() {
	t.hovered = false
	t.bg.FillColor = color.Transparent
	t.bg.Refresh()
}

type tappableIconRenderer struct {
	t *tappableIcon
}

func (r *tappableIconRenderer) Layout(size fyne.Size) {
	r.t.bg.Resize(size)
	pad := config.CurrentTheme.GetSize("copy_btn_padding")
	iconSize := fyne.NewSize(size.Width-pad*2, size.Height-pad*2)
	r.t.icon.Resize(iconSize)
	r.t.icon.Move(fyne.NewPos(pad, pad))
}

func (r *tappableIconRenderer) MinSize() fyne.Size {
	iconMin := r.t.icon.MinSize()
	pad := config.CurrentTheme.GetSize("copy_btn_padding")
	return fyne.NewSize(iconMin.Width+pad*2, iconMin.Height+pad*2)
}

func (r *tappableIconRenderer) Refresh() {
	r.t.bg.Refresh()
	r.t.icon.Refresh()
}

func (r *tappableIconRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.t.bg, r.t.icon}
}

func (r *tappableIconRenderer) Destroy() {}
