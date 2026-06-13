package ui

import (
	"bufio"
	"context"
	"image/color"
	"log"
	"os/exec"
	"time"

	"clipboard/config"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// ClipTheme wraps the default dark theme but substitutes DejaVu Sans
// so that Unicode symbols (arrows, etc.) render correctly, and custom
// colors matching the loaded config.
type ClipTheme struct{}

func (ClipTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return config.CurrentTheme.GetColor("background")
	case theme.ColorNameInputBackground:
		return config.CurrentTheme.GetColor("input_background")
	case theme.ColorNameButton:
		return config.CurrentTheme.GetColor("button")
	case theme.ColorNameHover:
		return config.CurrentTheme.GetColor("hover")
	case theme.ColorNameSelection:
		return config.CurrentTheme.GetColor("selection")
	case theme.ColorNameFocus:
		return config.CurrentTheme.GetColor("focus")
	case theme.ColorNamePrimary:
		return config.CurrentTheme.GetColor("primary")
	case theme.ColorNameScrollBar:
		return config.CurrentTheme.GetColor("scrollbar")
	case theme.ColorNameMenuBackground:
		return config.CurrentTheme.GetColor("menu_background")
	case theme.ColorNameOverlayBackground:
		return config.CurrentTheme.GetColor("overlay_background")
	case theme.ColorNameSeparator:
		return config.CurrentTheme.GetColor("separator")
	case theme.ColorNameForeground:
		return config.CurrentTheme.GetColor("foreground")
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return config.CurrentTheme.GetColor("placeholder")
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (ClipTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (ClipTheme) Font(style fyne.TextStyle) fyne.Resource {
	return resourceDejaVuSansTtf
}

func (ClipTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameInputRadius:
		return config.CurrentTheme.GetSize("input_radius")
	case theme.SizeNameSelectionRadius:
		return config.CurrentTheme.GetSize("selection_radius")
	}
	return theme.DefaultTheme().Size(name)
}

// StartThemeMonitor listens for system GTK theme changes and re-applies colors
// without requiring an application restart.
func StartThemeMonitor(ctx context.Context) {
	triggerReload := func() {
		fyne.Do(func() {
			config.LoadTheme()
			fyne.CurrentApp().Settings().SetTheme(ClipTheme{})
		})
	}

	// Monitor the entire interface schema with one process — simpler than
	// two separate per-key monitors, and handles any setting change.
	cmd := exec.CommandContext(ctx, "gsettings", "monitor", "org.gnome.desktop.interface")
	stdout, err := cmd.StdoutPipe()
	if err == nil {
		if err = cmd.Start(); err == nil {
			log.Println("Theme Monitor: using gsettings monitor backend")
			go func() {
				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					triggerReload()
				}
			}()
			return
		}
	}

	// Fallback to polling when gsettings monitor is unavailable.
	log.Println("Theme Monitor: falling back to 5s polling loop")
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		lastTheme := config.GTKThemeName()
		lastScheme := config.ColorScheme()

		for {
			select {
			case <-ticker.C:
				currTheme := config.GTKThemeName()
				currScheme := config.ColorScheme()
				if currTheme != lastTheme || currScheme != lastScheme {
					lastTheme = currTheme
					lastScheme = currScheme
					log.Printf("Theme Monitor: system theme changed to %s (%s)", currTheme, currScheme)
					triggerReload()
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}
