package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// clipTheme wraps the default dark theme but substitutes DejaVu Sans
// so that Unicode symbols (arrows, etc.) render correctly.
type clipTheme struct{}

func (clipTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}

func (clipTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (clipTheme) Font(style fyne.TextStyle) fyne.Resource {
	return resourceDejaVuSansTtf
}

func (clipTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
