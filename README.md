# clipd - Clipboard History Manager for Linux (Wayland/X11)

A lightweight clipboard history daemon with a searchable picker UI, built with Go and [Fyne](https://fyne.io). Designed for GNOME Wayland but also works on X11, Sway, and other compositors.

## Features

- Monitors the system clipboard and keeps a history of recent copies
- Searchable picker window triggered by a keyboard shortcut
- Keyboard navigation (arrow keys, Enter to select, Escape to close)
- Auto-paste: selected items are written to the clipboard and Ctrl+V is injected automatically
- System tray integration
- Deduplication and automatic expiry (200 entries, 24-hour max age)

## Architecture

```
main.go      - Entry point, daemon/trigger mode, PID file management
monitor.go   - Watches system clipboard for changes via golang.design/x/clipboard
history.go   - Thread-safe in-memory history store with dedup and eviction
hotkey.go    - Listens for SIGUSR1 signal to trigger the picker
ui.go        - Fyne-based picker window, search, system tray, paste simulation
```

### How it works

1. `clipd` (daemon mode) starts three subsystems:
   - **Monitor**: watches the clipboard for new text entries and stores them
   - **Hotkey listener**: waits for SIGUSR1 to show the picker
   - **UI**: runs the Fyne event loop with a hidden picker window

2. `clipd -trigger` (trigger mode) captures the active window ID, then sends SIGUSR1 to the running daemon

3. When triggered, the picker window appears with a search box and scrollable list. Selecting an entry writes it to the clipboard via `wl-copy` (Wayland) and simulates Ctrl+V using the first available tool: `ydotool` > `xdotool` > `wtype`

## Prerequisites

### Build dependencies (Debian/Ubuntu)

```bash
sudo apt install libgl1-mesa-dev xorg-dev libx11-dev
```

### Runtime dependencies

```bash
# Required - clipboard read on Wayland
sudo apt install wl-clipboard

# Required for auto-paste (at least one):
sudo apt install ydotool ydotoold   # Best option for GNOME Wayland
sudo apt install xdotool            # Fallback (XWayland apps only)
sudo apt install wtype              # For wlroots compositors (Sway)
```

## Build

```bash
go build -o clipd .
```

## Usage

### Start the daemon

```bash
./clipd
```

### Trigger the picker

From another terminal or a keyboard shortcut:

```bash
./clipd -trigger
```

### Bind a keyboard shortcut

**GNOME**: Settings > Keyboard > Custom Shortcuts > add:
```
Name:    Clipboard History
Command: /full/path/to/clipd -trigger
```

**i3/sway**:
```
bindsym $mod+v exec /full/path/to/clipd -trigger
```

**KDE**: System Settings > Shortcuts > Custom Shortcuts > Command/URL

## Troubleshooting

### ydotool: "ydotoold backend unavailable" / "failed to open uinput device"

This is the most common issue on GNOME Wayland. ydotool needs:

1. **Both packages installed** (Ubuntu splits them):
   ```bash
   sudo apt install ydotool ydotoold
   ```

2. **Your user in the `input` group**:
   ```bash
   sudo usermod -aG input $USER
   # Log out and back in for this to take effect
   ```

3. **uinput device accessible by the input group** (Ubuntu defaults to root-only):
   ```bash
   sudo bash -c 'echo "KERNEL==\"uinput\", GROUP=\"input\", MODE=\"0660\"" > /etc/udev/rules.d/80-uinput.rules'
   sudo udevadm control --reload-rules
   sudo udevadm trigger /dev/uinput
   ```

4. **ydotoold running as your user** (not root, otherwise the socket is root-only):
   ```bash
   ydotoold &
   ```

   Verify with:
   ```bash
   ydotool key ctrl+v 2>&1
   # Should print: "ydotool: notice: Using ydotoold backend"
   ```

### xdotool reports success but paste doesn't work

`xdotool` can only inject keystrokes into XWayland windows. If the target app is a native Wayland app (most GTK4/Qt6 apps on GNOME), xdotool keystrokes won't reach it. Use ydotool instead.

### prevWin is always empty

On GNOME Wayland, `xdotool getactivewindow` cannot detect the focused window. This is a Wayland security limitation. The picker still works - the compositor handles returning focus to the previous window when the picker hides.

### "Failed to read icon format image: unknown format"

This is a cosmetic Fyne warning about the app icon. It doesn't affect functionality.

## Autostart on Login

### Option 1: systemd user services (recommended)

Install the provided service files:

```bash
# Install service files
mkdir -p ~/.config/systemd/user
cp systemd/ydotoold.service ~/.config/systemd/user/
cp systemd/clipd.service ~/.config/systemd/user/

# Edit clipd.service to set the correct path to clipd binary
# (default assumes ~/Documents/go-projects/clipboard/clipd)

# Enable and start
systemctl --user daemon-reload
systemctl --user enable --now ydotoold
systemctl --user enable --now clipd
```

Check status:
```bash
systemctl --user status ydotoold
systemctl --user status clipd
journalctl --user -u clipd -f   # follow logs
```

### Option 2: GNOME Startup Applications

1. Open "Startup Applications" (or Tweaks > Startup Applications)
2. Add two entries:
   - **ydotoold**: command = `ydotoold`
   - **clipd**: command = `/full/path/to/clipd`

### Option 3: Shell profile

Add to `~/.bashrc` or `~/.profile`:
```bash
# Start ydotoold if not running
pgrep -x ydotoold >/dev/null || ydotoold &

# Start clipd if not running
pgrep -x clipd >/dev/null || /full/path/to/clipd &
```

## License

MIT
