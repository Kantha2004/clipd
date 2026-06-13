package config_test

import (
	"image/color"
	"testing"

	"clipboard/config"
)

// TestParseColor covers all documented input formats plus invalid fallback.
func TestParseColor(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  color.Color
	}{
		{
			name:  "named white",
			input: "white",
			want:  color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		},
		{
			name:  "named black",
			input: "black",
			want:  color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		},
		{
			name:  "3-char hex shorthand #abc",
			input: "#abc",
			// #abc → #aabbcc
			want: color.NRGBA{R: 0xaa, G: 0xbb, B: 0xcc, A: 255},
		},
		{
			name:  "3-char hex shorthand #fff",
			input: "#fff",
			want:  color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 255},
		},
		{
			name:  "6-char hex #aabbcc",
			input: "#aabbcc",
			want:  color.NRGBA{R: 0xaa, G: 0xbb, B: 0xcc, A: 255},
		},
		{
			name:  "6-char hex #e95420 (ubuntu orange)",
			input: "#e95420",
			want:  color.NRGBA{R: 0xe9, G: 0x54, B: 0x20, A: 255},
		},
		{
			name:  "6-char hex #000000",
			input: "#000000",
			want:  color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		},
		{
			name:  "6-char hex #ffffff",
			input: "#ffffff",
			want:  color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		},
		{
			name:  "rgba with float alpha 1.0",
			input: "rgba(255,255,255,1.0)",
			want:  color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		},
		{
			name:  "rgba with float alpha 0.0",
			input: "rgba(0,0,0,0.0)",
			want:  color.NRGBA{R: 0, G: 0, B: 0, A: 0},
		},
		{
			name:  "rgba with float alpha 0.5",
			input: "rgba(100,150,200,0.5)",
			// 0.5 * 255 = 127.5, truncated to uint8 → 127
			want: color.NRGBA{R: 100, G: 150, B: 200, A: 127},
		},
		{
			name:  "rgba with spaces",
			input: "rgba(255, 128, 0, 0.08)",
			// 0.08 * 255 = 20.4 → 20
			want: color.NRGBA{R: 255, G: 128, B: 0, A: 20},
		},
		{
			name:  "rgb without alpha",
			input: "rgb(10,20,30)",
			want:  color.NRGBA{R: 10, G: 20, B: 30, A: 255},
		},
		{
			name:  "rgb with spaces",
			input: "rgb(10, 20, 30)",
			want:  color.NRGBA{R: 10, G: 20, B: 30, A: 255},
		},
		{
			name:  "invalid string returns color.Transparent",
			input: "notacolor",
			want:  color.Transparent,
		},
		{
			name:  "empty string returns color.Transparent",
			input: "",
			want:  color.Transparent,
		},
		{
			name:  "invalid hex too short returns color.Transparent",
			input: "#zz",
			want:  color.Transparent,
		},
		{
			name:  "leading and trailing spaces are trimmed",
			input: "  white  ",
			want:  color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := config.ParseColor(tc.input)
			if got != tc.want {
				t.Errorf("ParseColor(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestGsettingsClean covers stripping single quotes from gsettings output.
func TestGsettingsClean(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "quoted value",
			input: "'Yaru-dark'",
			want:  "Yaru-dark",
		},
		{
			name:  "quoted with surrounding spaces",
			input: "  'x'  ",
			want:  "x",
		},
		{
			name:  "unquoted value passes through",
			input: "noquotes",
			want:  "noquotes",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only spaces",
			input: "   ",
			want:  "",
		},
		{
			name:  "single quote only at start (no trailing quote) passes through",
			input: "'noend",
			want:  "'noend",
		},
		{
			name:  "single quote only at end (no leading quote) passes through",
			input: "nostart'",
			want:  "nostart'",
		},
		{
			name:  "quoted with newline (gsettings appends newline)",
			input: "'prefer-dark'\n",
			want:  "prefer-dark",
		},
		{
			name:  "quoted empty string",
			input: "''",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := config.GsettingsClean(tc.input)
			if got != tc.want {
				t.Errorf("GsettingsClean(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestParseGTKCSS covers @define-color parsing, comment stripping, and variable resolution.
func TestParseGTKCSS(t *testing.T) {
	t.Run("basic @define-color", func(t *testing.T) {
		css := `
@define-color bg_color #2d2d2d;
@define-color fg_color #ffffff;
`
		got := config.ParseGTKCSS(css)
		if got["bg_color"] != "#2d2d2d" {
			t.Errorf("bg_color = %q, want %q", got["bg_color"], "#2d2d2d")
		}
		if got["fg_color"] != "#ffffff" {
			t.Errorf("fg_color = %q, want %q", got["fg_color"], "#ffffff")
		}
	})

	t.Run("ignores non-define-color lines", func(t *testing.T) {
		css := `
.some-class { color: red; }
@define-color accent #e95420;
* { margin: 0; }
`
		got := config.ParseGTKCSS(css)
		if len(got) != 1 {
			t.Errorf("expected 1 entry, got %d: %v", len(got), got)
		}
		if got["accent"] != "#e95420" {
			t.Errorf("accent = %q, want %q", got["accent"], "#e95420")
		}
	})

	t.Run("inline comment stripping", func(t *testing.T) {
		css := `@define-color primary #e95420; /* ubuntu orange */`
		got := config.ParseGTKCSS(css)
		if got["primary"] != "#e95420" {
			t.Errorf("primary = %q, want %q", got["primary"], "#e95420")
		}
	})

	t.Run("multiple inline comments on same line", func(t *testing.T) {
		css := `@define-color /* skip this */ base_color /* also this */ #1e1e1e;`
		got := config.ParseGTKCSS(css)
		if got["base_color"] != "#1e1e1e" {
			t.Errorf("base_color = %q, want %q", got["base_color"], "#1e1e1e")
		}
	})

	t.Run("one-level @variable reference resolution", func(t *testing.T) {
		css := `
@define-color base #2d2d2d;
@define-color background @base;
`
		got := config.ParseGTKCSS(css)
		if got["background"] != "#2d2d2d" {
			t.Errorf("background = %q, want %q (after resolving @base)", got["background"], "#2d2d2d")
		}
	})

	t.Run("two-level @variable reference resolution", func(t *testing.T) {
		css := `
@define-color root #ffffff;
@define-color mid @root;
@define-color top @mid;
`
		got := config.ParseGTKCSS(css)
		if got["top"] != "#ffffff" {
			t.Errorf("top = %q, want %q (after resolving @mid → @root)", got["top"], "#ffffff")
		}
		if got["mid"] != "#ffffff" {
			t.Errorf("mid = %q, want %q (after resolving @root)", got["mid"], "#ffffff")
		}
	})

	t.Run("empty CSS returns empty map", func(t *testing.T) {
		got := config.ParseGTKCSS("")
		if len(got) != 0 {
			t.Errorf("expected empty map, got %v", got)
		}
	})

	t.Run("unresolved reference stays as-is", func(t *testing.T) {
		css := `@define-color orphan @nonexistent;`
		got := config.ParseGTKCSS(css)
		// @nonexistent is never defined, so orphan stays as "@nonexistent"
		if got["orphan"] != "@nonexistent" {
			t.Errorf("orphan = %q, want %q", got["orphan"], "@nonexistent")
		}
	})
}

// TestAdjustBrightness covers positive/negative amounts and clamping at 0/255.
func TestAdjustBrightness(t *testing.T) {
	tests := []struct {
		name   string
		input  color.NRGBA
		amount int
		want   color.NRGBA
	}{
		{
			name:   "positive amount increases channels",
			input:  color.NRGBA{R: 100, G: 100, B: 100, A: 255},
			amount: 20,
			want:   color.NRGBA{R: 120, G: 120, B: 120, A: 255},
		},
		{
			name:   "negative amount decreases channels",
			input:  color.NRGBA{R: 100, G: 100, B: 100, A: 255},
			amount: -20,
			want:   color.NRGBA{R: 80, G: 80, B: 80, A: 255},
		},
		{
			name:   "clamp at 255 when overflow",
			input:  color.NRGBA{R: 250, G: 250, B: 250, A: 255},
			amount: 10,
			want:   color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		},
		{
			name:   "clamp at 0 when underflow",
			input:  color.NRGBA{R: 5, G: 5, B: 5, A: 255},
			amount: -10,
			want:   color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		},
		{
			name:   "alpha is preserved unchanged",
			input:  color.NRGBA{R: 128, G: 128, B: 128, A: 100},
			amount: 0,
			want:   color.NRGBA{R: 128, G: 128, B: 128, A: 100},
		},
		{
			name:   "alpha is preserved when amount is positive",
			input:  color.NRGBA{R: 50, G: 60, B: 70, A: 200},
			amount: 5,
			want:   color.NRGBA{R: 55, G: 65, B: 75, A: 200},
		},
		{
			name:   "zero amount leaves color unchanged",
			input:  color.NRGBA{R: 128, G: 64, B: 32, A: 255},
			amount: 0,
			want:   color.NRGBA{R: 128, G: 64, B: 32, A: 255},
		},
		{
			name:   "large positive clamps all channels",
			input:  color.NRGBA{R: 10, G: 20, B: 30, A: 255},
			amount: 300,
			want:   color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		},
		{
			name:   "large negative clamps all channels",
			input:  color.NRGBA{R: 200, G: 150, B: 100, A: 255},
			amount: -300,
			want:   color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := config.AdjustBrightness(tc.input, tc.amount)
			if got != tc.want {
				t.Errorf("AdjustBrightness(%v, %d) = %v, want %v", tc.input, tc.amount, got, tc.want)
			}
		})
	}
}

// TestThemeConfigGetColor covers found keys and missing key fallback.
func TestThemeConfigGetColor(t *testing.T) {
	theme := &config.ThemeConfig{
		Colors: map[string]string{
			"primary":   "#e95420",
			"secondary": "rgb(0,128,255)",
		},
		Sizes: map[string]float32{},
	}

	t.Run("found key returns parsed color", func(t *testing.T) {
		want := color.NRGBA{R: 0xe9, G: 0x54, B: 0x20, A: 255}
		got := theme.GetColor("primary")
		if got != want {
			t.Errorf("GetColor(\"primary\") = %v, want %v", got, want)
		}
	})

	t.Run("found key with rgb value", func(t *testing.T) {
		want := color.NRGBA{R: 0, G: 128, B: 255, A: 255}
		got := theme.GetColor("secondary")
		if got != want {
			t.Errorf("GetColor(\"secondary\") = %v, want %v", got, want)
		}
	})

	t.Run("missing key returns color.Transparent", func(t *testing.T) {
		got := theme.GetColor("nonexistent")
		if got != color.Transparent {
			t.Errorf("GetColor(\"nonexistent\") = %v, want color.Transparent", got)
		}
	})
}

// TestThemeConfigGetSize covers found keys and missing key fallback.
func TestThemeConfigGetSize(t *testing.T) {
	theme := &config.ThemeConfig{
		Colors: map[string]string{},
		Sizes: map[string]float32{
			"window_width":  620,
			"input_radius":  6.0,
			"badge_radius":  3.0,
		},
	}

	t.Run("found key returns size", func(t *testing.T) {
		got := theme.GetSize("window_width")
		if got != 620 {
			t.Errorf("GetSize(\"window_width\") = %v, want 620", got)
		}
	})

	t.Run("found key with fractional value", func(t *testing.T) {
		got := theme.GetSize("input_radius")
		if got != 6.0 {
			t.Errorf("GetSize(\"input_radius\") = %v, want 6.0", got)
		}
	})

	t.Run("found key with small value", func(t *testing.T) {
		got := theme.GetSize("badge_radius")
		if got != 3.0 {
			t.Errorf("GetSize(\"badge_radius\") = %v, want 3.0", got)
		}
	})

	t.Run("missing key returns 0.0", func(t *testing.T) {
		got := theme.GetSize("nonexistent")
		if got != 0.0 {
			t.Errorf("GetSize(\"nonexistent\") = %v, want 0.0", got)
		}
	})
}
