package ui_test

import (
	"fmt"
	"testing"
	"time"

	"clipboard/ui"
)

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "30 seconds ago",
			duration: 30 * time.Second,
			want:     "30s ago",
		},
		{
			name:     "5 minutes ago",
			duration: 5 * time.Minute,
			want:     "5m ago",
		},
		{
			name:     "3 hours ago",
			duration: 3 * time.Hour,
			want:     "3h ago",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ui.FormatAge(time.Now().Add(-tc.duration))
			if got != tc.want {
				t.Errorf("FormatAge(%v ago) = %q, want %q", tc.duration, got, tc.want)
			}
		})
	}

	t.Run("2 days ago uses Jan 2 format", func(t *testing.T) {
		ts := time.Now().Add(-48 * time.Hour)
		got := ui.FormatAge(ts)
		want := ts.Format("Jan 2")
		if got != want {
			t.Errorf("FormatAge(2 days ago) = %q, want %q", got, want)
		}
		// Sanity-check the format looks like "Jan D" or "Jan DD"
		var month string
		var day int
		if _, err := fmt.Sscanf(got, "%s %d", &month, &day); err != nil {
			t.Errorf("FormatAge(2 days ago) = %q does not match 'Mon D' layout: %v", got, err)
		}
	})
}

func TestSingleLinePreview(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "empty string",
			content: "",
			want:    "",
		},
		{
			name:    "whitespace only",
			content: "   \t\n  ",
			want:    "",
		},
		{
			name:    "single short line",
			content: "hello world",
			want:    "hello world",
		},
		{
			name:    "multi-line returns only first line",
			content: "first line\nsecond line\nthird line",
			want:    "first line",
		},
		{
			name:    "line longer than 50 runes is truncated with ellipsis",
			content: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			want:    "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWX" + "...",
		},
		{
			name:    "leading and trailing whitespace on first line is trimmed",
			content: "  trimmed content  ",
			want:    "trimmed content",
		},
		{
			name: "unicode content beyond 50 runes truncated by rune count",
			// Each Japanese character is one rune but multiple bytes.
			// 55 runes total — should truncate after rune 50.
			// あ(1)い(2)う(3)え(4)お(5)か(6)き(7)く(8)け(9)こ(10)
			// さ(11)し(12)す(13)せ(14)そ(15)た(16)ち(17)つ(18)て(19)と(20)
			// な(21)に(22)ぬ(23)ね(24)の(25)は(26)ひ(27)ふ(28)へ(29)ほ(30)
			// ま(31)み(32)む(33)め(34)も(35)や(36)ゆ(37)よ(38)ら(39)り(40)
			// る(41)れ(42)ろ(43)わ(44)を(45)ん(46)が(47)ぎ(48)ぐ(49)げ(50)
			// ご(51)ざ(52)じ(53)ず(54)ぜ(55)
			content: "あいうえおかきくけこさしすせそたちつてとなにぬねのはひふへほまみむめもやゆよらりるれろわをんがぎぐげござじずぜ",
			want:    string([]rune("あいうえおかきくけこさしすせそたちつてとなにぬねのはひふへほまみむめもやゆよらりるれろわをんがぎぐげござじずぜ")[:50]) + "...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ui.SingleLinePreview(tc.content)
			if got != tc.want {
				t.Errorf("SingleLinePreview(%q) = %q, want %q", tc.content, got, tc.want)
			}
		})
	}
}
