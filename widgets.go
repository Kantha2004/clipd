package main

import (
	"image/color"

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

var colorHover = color.NRGBA{R: 255, G: 255, B: 255, A: 30}

// tappableContainer wraps any CanvasObject to make it clickable and hoverable.
type tappableContainer struct {
	widget.BaseWidget
	content  fyne.CanvasObject
	bg       *canvas.Rectangle
	ts       *widget.Label
	onTapped func(shiftHeld bool)
}

func newTappableContainer(content fyne.CanvasObject, bg *canvas.Rectangle, ts *widget.Label, onTapped func(shiftHeld bool)) *tappableContainer {
	t := &tappableContainer{content: content, bg: bg, ts: ts, onTapped: onTapped}
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
