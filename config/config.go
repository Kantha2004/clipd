package config

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const themeFilename = "theme.json"

type ThemeConfig struct {
	Colors map[string]string  `json:"colors"`
	Sizes  map[string]float32 `json:"sizes"`
}

var CurrentTheme = defaultTheme()

var (
	ColorSelected color.Color = color.NRGBA{R: 69, G: 69, B: 69, A: 255}
	ColorHover    color.Color = color.NRGBA{R: 50, G: 50, B: 50, A: 255}
	ColorDefault  color.Color = color.Transparent
)

func defaultTheme() *ThemeConfig {
	return &ThemeConfig{
		Colors: map[string]string{
			"background":         "#242424",
			"input_background":   "#1e1e1e",
			"button":             "#303030",
			"hover":              "#323232",
			"selection":          "#454545",
			"focus":              "#e95420",
			"primary":            "#e95420",
			"scrollbar":          "#303030",
			"menu_background":    "#1e1e1e",
			"overlay_background": "#1e1e1e",
			"separator":          "rgba(255,255,255,0.08)",
			"foreground":         "#f3f4f6",
			"placeholder":        "#aeafb0",
			"keycap_border":      "#383838",
			"badge_cmd_bg":       "#0c251e",
			"badge_cmd_fg":       "#10b981",
			"badge_link_bg":      "#121e36",
			"badge_link_fg":      "#3b82f6",
			"badge_image_bg":     "#3c1032",
			"badge_image_fg":     "#e879f9",
			"badge_video_bg":     "#2c1616",
			"badge_video_fg":     "#ef4444",
			"badge_file_bg":      "#202732",
			"badge_file_fg":      "#94a3b8",
			"copy_btn_hover":     "rgba(255,255,255,0.08)",
			"copy_btn_flash":     "rgba(110,89,255,0.40)",
		},
		Sizes: map[string]float32{
			"window_width":     620,
			"window_height":    440,
			"input_radius":     6.0,
			"selection_radius": 6.0,
			"keycap_radius":    4.0,
			"badge_radius":     3.0,
			"copy_btn_radius":  3.0,
			"keycap_padding_x": 12.0,
			"keycap_padding_y": 6.0,
			"copy_btn_size":    24.0,
			"copy_btn_padding": 4.0,
		},
	}
}

func LoadTheme() {
	defaults := defaultTheme()

	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("error getting home dir: %v", err)
		return
	}
	configDir := filepath.Join(home, ".config", "clipd")
	configPath := filepath.Join(configDir, themeFilename)

	// Try local theme.json first (useful for dev)
	if _, err := os.Stat(themeFilename); err == nil {
		configPath = themeFilename
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		os.MkdirAll(configDir, 0755)
		if data, err := json.MarshalIndent(defaults, "", "  "); err == nil {
			os.WriteFile(configPath, data, 0644)
			log.Printf("Created default theme config at %s", configPath)
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("using default theme: could not read config: %v", err)
		return
	}

	var theme ThemeConfig
	if err := json.Unmarshal(data, &theme); err != nil {
		log.Printf("using default theme: invalid json: %v", err)
		return
	}

	// Merge with defaults to ensure all keys exist.
	for k, v := range defaults.Colors {
		if _, ok := theme.Colors[k]; !ok {
			theme.Colors[k] = v
		}
	}
	for k, v := range defaults.Sizes {
		if _, ok := theme.Sizes[k]; !ok {
			theme.Sizes[k] = v
		}
	}

	CurrentTheme = &theme

	// Try loading system colors on top of the config theme
	loadSystemColors()

	// Update globals
	ColorSelected = CurrentTheme.GetColor("selection")
	ColorHover = CurrentTheme.GetColor("hover")

	log.Printf("Loaded theme from %s (with system color overrides if available)", configPath)
}

func ParseColor(s string) color.Color {
	s = strings.TrimSpace(s)
	if s == "white" {
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	if s == "black" {
		return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	}
	if strings.HasPrefix(s, "#") {
		s = strings.TrimPrefix(s, "#")
		if len(s) == 3 {
			s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
		}
		if len(s) == 6 {
			var r, g, b uint8
			_, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b)
			if err == nil {
				return color.NRGBA{R: r, G: g, B: b, A: 255}
			}
		}
	} else if strings.HasPrefix(s, "rgba(") {
		sNoSpace := strings.ReplaceAll(s, " ", "")
		var r, g, b int
		var a float32
		_, err := fmt.Sscanf(sNoSpace, "rgba(%d,%d,%d,%f)", &r, &g, &b, &a)
		if err == nil {
			return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a * 255)}
		}
	} else if strings.HasPrefix(s, "rgb(") {
		sNoSpace := strings.ReplaceAll(s, " ", "")
		var r, g, b int
		_, err := fmt.Sscanf(sNoSpace, "rgb(%d,%d,%d)", &r, &g, &b)
		if err == nil {
			return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
		}
	}
	return color.Transparent
}

func (t *ThemeConfig) GetColor(name string) color.Color {
	if val, ok := t.Colors[name]; ok {
		return ParseColor(val)
	}
	return color.Transparent
}

func (t *ThemeConfig) GetSize(name string) float32 {
	if val, ok := t.Sizes[name]; ok {
		return val
	}
	return 0.0
}

func GTKThemeName() string {
	out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "gtk-theme").Output()
	if err != nil {
		return ""
	}
	return GsettingsClean(string(out))
}

func ColorScheme() string {
	out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "color-scheme").Output()
	if err != nil {
		return ""
	}
	return GsettingsClean(string(out))
}

func GsettingsClean(val string) string {
	val = strings.TrimSpace(val)
	if len(val) >= 2 && val[0] == '\'' && val[len(val)-1] == '\'' {
		return val[1 : len(val)-1]
	}
	return val
}

func extractCSSFromGResource(gresourcePath, themeName string) string {
	cmd := exec.Command("gresource", "list", gresourcePath)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(out), "\n")
	var darkPath, normalPath string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "gtk-dark.css") {
			darkPath = line
		} else if strings.HasSuffix(line, "gtk.css") {
			normalPath = line
		}
	}

	resourcePath := normalPath
	isDark := strings.Contains(strings.ToLower(themeName), "dark") || ColorScheme() == "prefer-dark"
	if isDark && darkPath != "" {
		resourcePath = darkPath
	} else if resourcePath == "" && darkPath != "" {
		resourcePath = darkPath
	}

	if resourcePath == "" {
		return ""
	}

	cmdExtract := exec.Command("gresource", "extract", gresourcePath, resourcePath)
	cssOut, err := cmdExtract.Output()
	if err != nil {
		return ""
	}
	return string(cssOut)
}

func ParseGTKCSS(cssContent string) map[string]string {
	colors := make(map[string]string)
	lines := strings.Split(cssContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Strip inline comments /* ... */
		for {
			start := strings.Index(line, "/*")
			if start == -1 {
				break
			}
			end := strings.Index(line[start:], "*/")
			if end == -1 {
				break
			}
			line = line[:start] + line[start+end+2:]
		}
		line = strings.TrimSpace(line)

		if !strings.HasPrefix(line, "@define-color") {
			continue
		}

		parts := strings.Fields(line[len("@define-color"):])
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		val := strings.Join(parts[1:], " ")
		val = strings.TrimSuffix(val, ";")
		colors[name] = val
	}

	// Resolve variable references (up to 5 levels deep)
	for i := 0; i < 5; i++ {
		resolvedAny := false
		for k, v := range colors {
			if strings.HasPrefix(v, "@") {
				refKey := strings.TrimPrefix(v, "@")
				if refVal, found := colors[refKey]; found && !strings.HasPrefix(refVal, "@") {
					colors[k] = refVal
					resolvedAny = true
				}
			}
		}
		if !resolvedAny {
			break
		}
	}

	return colors
}

func AdjustBrightness(c color.NRGBA, amount int) color.NRGBA {
	adj := func(val uint8) uint8 {
		res := int(val) + amount
		if res < 0 {
			return 0
		}
		if res > 255 {
			return 255
		}
		return uint8(res)
	}
	return color.NRGBA{R: adj(c.R), G: adj(c.G), B: adj(c.B), A: c.A}
}

func loadSystemColors() {
	themeName := GTKThemeName()
	if themeName == "" {
		return
	}

	home, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(home, ".themes"),
		filepath.Join(home, ".local", "share", "themes"),
		"/usr/share/themes",
	}

	var themeDir string
	for _, d := range dirs {
		td := filepath.Join(d, themeName)
		if _, err := os.Stat(td); err == nil {
			themeDir = td
			break
		}
	}
	if themeDir == "" {
		fallbackName := themeName
		if strings.HasSuffix(themeName, "-dark") {
			fallbackName = strings.TrimSuffix(themeName, "-dark")
		} else {
			fallbackName = themeName + "-dark"
		}
		for _, d := range dirs {
			td := filepath.Join(d, fallbackName)
			if _, err := os.Stat(td); err == nil {
				themeDir = td
				break
			}
		}
	}

	if themeDir == "" {
		log.Printf("System theme directory not found for %s", themeName)
		return
	}

	gtkDir := filepath.Join(themeDir, "gtk-3.0")
	var cssContent string

	// Try gresource first
	gresourcePath := filepath.Join(gtkDir, "gtk.gresource")
	if _, err := os.Stat(gresourcePath); err == nil {
		cssContent = extractCSSFromGResource(gresourcePath, themeName)
	}

	// If no CSS from gresource, try direct css files
	if cssContent == "" {
		for _, cssFile := range []string{"gtk-dark.css", "gtk.css"} {
			p := filepath.Join(gtkDir, cssFile)
			if data, err := os.ReadFile(p); err == nil {
				cssContent = string(data)
				break
			}
		}
	}

	if cssContent == "" {
		log.Printf("No CSS content found for theme %s", themeName)
		return
	}

	sysColors := ParseGTKCSS(cssContent)
	if len(sysColors) == 0 {
		return
	}

	log.Printf("Successfully parsed %d system theme colors from GTK theme %s", len(sysColors), themeName)

	setColor := func(targetKey string, sysKeys []string) {
		for _, sysKey := range sysKeys {
			if val, ok := sysColors[sysKey]; ok {
				val = strings.TrimSpace(val)
				if ParseColor(val) != color.Transparent {
					CurrentTheme.Colors[targetKey] = val
					break
				}
			}
		}
	}

	setColor("background", []string{"theme_bg_color"})
	setColor("input_background", []string{"theme_base_color", "theme_bg_color"})
	setColor("menu_background", []string{"theme_base_color", "theme_bg_color"})
	setColor("overlay_background", []string{"theme_base_color", "theme_bg_color"})
	setColor("selection", []string{"theme_selected_bg_color"})
	setColor("focus", []string{"theme_selected_bg_color"})
	setColor("primary", []string{"theme_selected_bg_color"})
	setColor("foreground", []string{"theme_text_color", "theme_fg_color"})
	setColor("placeholder", []string{"insensitive_fg_color", "theme_unfocused_fg_color"})
	setColor("separator", []string{"borders", "unfocused_borders"})

	// Adjust hover and button based on background brightness.
	if bg, ok := CurrentTheme.GetColor("background").(color.NRGBA); ok && bg.A > 0 {
		brightness := (int(bg.R) + int(bg.G) + int(bg.B)) / 3
		hoverAmt, btnAmt := 12, 8
		if brightness >= 128 {
			hoverAmt, btnAmt = -12, -8
		}
		nrgbaToCSS := func(c color.NRGBA) string {
			return fmt.Sprintf("rgba(%d,%d,%d,%.3f)", c.R, c.G, c.B, float32(c.A)/255.0)
		}
		if _, ok := sysColors["theme_hover_color"]; !ok {
			CurrentTheme.Colors["hover"] = nrgbaToCSS(AdjustBrightness(bg, hoverAmt))
		}
		if _, ok := sysColors["theme_button_color"]; !ok {
			CurrentTheme.Colors["button"] = nrgbaToCSS(AdjustBrightness(bg, btnAmt))
		}
	}
}
