package core

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

var PrevWinFile = filepath.Join(os.TempDir(), "clipd.prevwin")

// RunHotkey listens for SIGUSR1 and calls onTrigger each time it fires.
//
// To trigger the picker, bind any key in your DE to:
//
//	pkill -USR1 clipd
//
// GNOME:  Settings → Keyboard → Custom Shortcuts → add the command above
// KDE:    System Settings → Shortcuts → Custom Shortcuts → Command/URL
// i3/sway: bindsym $mod+v exec pkill -USR1 clipd
func RunHotkey(ctx context.Context, onTrigger func(prevWindowID string)) error {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	defer signal.Stop(ch)

	log.Println("waiting for trigger — bind a key in your DE to: clipd -trigger")
	for {
		select {
		case <-ch:
			log.Println("SIGUSR1 received — showing picker")
			onTrigger(ReadPrevWin())
		case <-ctx.Done():
			return nil
		}
	}
}

// readPrevWin reads the window ID written by `clipd -trigger` at key-press time.
func ReadPrevWin() string {
	data, err := os.ReadFile(PrevWinFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
