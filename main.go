// clipd – clipboard history daemon.
//
// Build dependencies (Debian/Ubuntu):
//
//	sudo apt install libgl1-mesa-dev xorg-dev libx11-dev
//
// Runtime dependency for auto-paste (optional):
//
//	sudo apt install xdotool
//
// Usage:
//
//	clipd          – start the daemon
//	clipd -trigger – open the picker in a running daemon
//
// DE shortcut setup (bind your chosen key to):
//
//	/path/to/clipd -trigger
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"fyne.io/fyne/v2/app"
)

var (
	pidFile     = filepath.Join(os.TempDir(), "clipd.pid")
	prevWinFile = filepath.Join(os.TempDir(), "clipd.prevwin")
)

func main() {
	log.SetPrefix("[clipd] ")

	trigger := flag.Bool("trigger", false, "open the picker in a running clipd daemon")
	flag.Parse()

	if *trigger {
		runTrigger()
		return
	}

	runDaemon()
}

// runTrigger signals the running daemon to show the picker.
// It tries the PID file first, then falls back to pgrep.
func runTrigger() {
	pid := pidFromFile()
	if pid == 0 {
		pid = pidFromPgrep()
	}
	if pid == 0 {
		fmt.Fprintln(os.Stderr, "clipd does not appear to be running")
		os.Exit(1)
	}

	// Capture active window before focus shifts to the picker.
	if out, xerr := exec.Command("xdotool", "getactivewindow").Output(); xerr == nil {
		os.WriteFile(prevWinFile, []byte(strings.TrimSpace(string(out))), 0o644)
	} else {
		os.Remove(prevWinFile)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintln(os.Stderr, "process not found:", pid)
		os.Exit(1)
	}
	if err := proc.Signal(syscall.SIGUSR1); err != nil {
		fmt.Fprintln(os.Stderr, "signal failed:", err)
		os.Exit(1)
	}
}

func pidFromFile() int {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

func pidFromPgrep() int {
	out, err := exec.Command("pgrep", "-x", "clipd").Output()
	if err != nil {
		return 0
	}
	// pgrep can return multiple lines; take the first that isn't us.
	self := os.Getpid()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && pid != self {
			return pid
		}
	}
	return 0
}

func runDaemon() {
	// Write PID file so `clipd -trigger` can find us.
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		log.Printf("warning: could not write PID file: %v", err)
	}
	defer os.Remove(pidFile)

	store := NewHistoryStore(24*time.Hour, 200)

	a := app.NewWithID("io.github.clipd")
	ui := NewUI(a, store)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := RunMonitor(ctx, store); err != nil && ctx.Err() == nil {
			log.Printf("monitor stopped: %v", err)
		}
	}()

	go func() {
		if err := RunHotkey(ctx, ui.ShowPicker); err != nil && ctx.Err() == nil {
			log.Printf("hotkey stopped: %v", err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
		a.Quit()
	}()

	ui.Run()
	cancel()
}
