# clipd — Clipboard History Manager for Linux

A lightweight clipboard history daemon with a searchable picker UI, built with Go and [Fyne](https://fyne.io). Designed for GNOME Wayland but also works on X11, Sway, and other compositors.

## Features

- **GTK Theme Auto-Sync**: Automatically synchronizes the picker interface colors with your system's active GTK theme and accent color (Ubuntu Yaru Orange, Red, Blue, Purple, etc.) and dark/light modes.
- **Smart Mimetype Previews**: Detects copied file paths on the fly (e.g., PDFs, scripts, code, archives) and displays custom mimetype icons matching your active desktop icon theme (Yaru or Adwaita fallbacks).
- **Raw Image Support**: Saves raw image clipboard copies (like screenshots) and generates local image thumbnails under `~/.config/clipd/media/` for rendering in the picker.
- **Automatic Garbage Collection**: Cleans up generated media and thumbnail files when clips are deleted or evicted (due to size or age limits) and runs an asynchronous scan on startup to delete orphaned media files.
- **Up-Arrow Search Focus**: Seamless keyboard navigation that returns focus to the search bar and unselects list items when pressing the Up arrow from the first item in history.
- **Polished Row Selection**: Features a 5px accent-colored left indicator bar on selection, using a subtle row hover background to feel like a premium, native GTK application.
- **Automated Auto-Paste**: Selected items are written to the clipboard and the paste key is injected automatically into the target window (with smart terminal fallback to `Ctrl+Shift+V`).
- **Single-instance Guard**: Launching a new daemon process gracefully kills and replaces the existing active instance to prevent resource conflicts.
- **System Tray Integration**: Full system tray menu for quick history picker access, clear history, and pause/resume monitoring.

---

## Install

### Option A — PPA (Preferred)

```bash
sudo add-apt-repository ppa:kantha2004/clipd
sudo apt update
sudo apt install clipd
```

The installer automatically:
- Enables and starts the `clipd` systemd user service
- Registers **Super+V** as the keyboard shortcut (GNOME only)

### Option B — Download `.deb` (Alternative)

Download the latest `clipd_*_amd64.deb` from the [Releases](../../releases) page:

```bash
sudo dpkg -i clipd_*_amd64.deb
```

Same automatic setup as Option A.

### Option C — Build from source

**Build dependencies:**

```bash
sudo apt install libgl1-mesa-dev xorg-dev libx11-dev
```

```bash
git clone https://github.com/kantha2004/clipd
cd clipd
make enable        # build + install to ~/.local/bin + enable systemd services + register shortcut
```

---

## Runtime dependencies

```bash
# Required — clipboard access on Wayland
sudo apt install wl-clipboard

# Required for auto-paste (at least one):
sudo apt install ydotool ydotoold   # best option for GNOME Wayland (uinput-level)
sudo apt install xdotool            # fallback for XWayland apps
sudo apt install wtype              # for wlroots compositors (Sway, Hyprland)
```

---

## Usage

### Keyboard shortcut

On **GNOME**, the **Super+V** shortcut is registered automatically during installation.

For other desktop environments, bind your chosen key to `clipd -trigger` manually:

**i3 / Sway** — add to your config file:
```
bindsym $mod+v exec clipd -trigger
```

**KDE** — System Settings → Shortcuts → Custom Shortcuts → Command/URL

### Picker controls

| Key / Action | Effect |
|---|---|
| Type | Filter history |
| ↑ / ↓ | Navigate items |
| Enter | Paste selected item (Ctrl+V) |
| Shift+Enter | Paste selected item (Ctrl+Shift+V — for terminals) |
| Click | Paste item (Ctrl+V) |
| Shift+Click | Paste item (Ctrl+Shift+V — for terminals) |
| Copy icon | Copy to clipboard without pasting |
| Escape | Close picker |

---

## Make targets

| Target | Description |
|---|---|
| `make build` | Compile binary to `bin/clipd` |
| `make install` | Install to `~/.local/bin` and install systemd services |
| `make enable` | Install + enable and start systemd services + register GNOME shortcut |
| `make uninstall` | Disable services and remove installed files |
| `make deb` | Build a `.deb` package in `bin/` |
| `make deb VERSION=1.2.0` | Build `.deb` with a specific version |
| `make source` | Build signed Debian source package for PPA upload |
| `make ppa` | Upload source package to Launchpad PPA |
| `make clean` | Remove `bin/` and `vendor/` |

---

## Architecture

```
main.go       Entry point, daemon/trigger mode, PID file, single-instance guard
monitor.go    Watches system clipboard for changes (golang.design/x/clipboard)
history.go    Thread-safe in-memory history store with dedup and eviction
hotkey.go     Listens for SIGUSR1 to show the picker
ui.go         Fyne picker window, search, system tray, paste simulation
widgets.go    Reusable Fyne widgets: navEntry (keyboard-nav search box),
              tappableContainer (clickable + hoverable list row)
theme.go      Custom Fyne theme
```

### How it works

1. `clipd` (daemon mode) starts three subsystems:
   - **Monitor** — polls the clipboard and stores new entries
   - **Hotkey listener** — waits for SIGUSR1 to show the picker
   - **UI** — runs the Fyne event loop with a hidden picker window

2. `clipd -trigger` (trigger mode) captures the currently active window ID, then sends SIGUSR1 to the running daemon

3. When triggered, the picker appears. Selecting an entry:
   - Writes the content to the clipboard via `wl-copy` (or X11 fallback)
   - Refocuses the previously active window via `xdotool windowfocus`
   - Injects the paste key (`Ctrl+V` or `Ctrl+Shift+V`) using `ydotool` → `xdotool` → `wtype`

---

## Troubleshooting

### ydotool: "backend unavailable" / "failed to open uinput"

1. **Install both packages** (Ubuntu splits the daemon):
   ```bash
   sudo apt install ydotool ydotoold
   ```

2. **Add your user to the `input` group** and re-login:
   ```bash
   sudo usermod -aG input $USER
   ```

3. **Allow `input` group to access `/dev/uinput`**:
   ```bash
   sudo bash -c 'echo "KERNEL==\"uinput\", GROUP=\"input\", MODE=\"0660\"" \
     > /etc/udev/rules.d/80-uinput.rules'
   sudo udevadm control --reload-rules
   sudo udevadm trigger /dev/uinput
   ```

4. **Run ydotoold as your user** (not root):
   ```bash
   systemctl --user enable --now ydotoold
   ```

   Verify:
   ```bash
   ydotool key ctrl+v
   # Should print: "ydotool: notice: Using ydotoold backend"
   ```

### xdotool paste doesn't work

`xdotool` can only inject keystrokes into XWayland windows. Native Wayland apps (GTK4, Qt6 on GNOME) require ydotool.

### Active window not restored after picking

On GNOME Wayland, `xdotool getactivewindow` cannot detect the focused window — this is a Wayland security restriction. The compositor will return focus to the previous window automatically when the picker closes.

---

## License

MIT

---

<a target="_blank" href="https://www.svgrepo.com/svg/257864/clipboard-tasks">Clipboard</a> icon by <a target="_blank" href="https://www.svgrepo.com">SVG Repo</a>
