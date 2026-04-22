# Turing Smart Screen — Agent Handbook

## Project Overview

**Purpose:** Drive a 3.5" USB-C Turing Smart Screen (320×480 portrait) via serial to display live system stats (CPU, RAM, GPU, Disk, Network) and a scrollable message queue with word-wrapped text.

**Location:** `/home/evndr/dev/screen/`
**Language:** Go 1.25+
**OS:** Linux (6.x kernel tested)

---

## Hardware

| Property | Value |
|----------|-------|
| Display | 3.5" IPS LCD, 320×480 px (portrait default) |
| MCU | WCH CH552T 8-bit E8051 core |
| Connection | USB-C → virtual serial (115200 baud, 8N1) |
| USB VID:PID | `1a86:5722` (WCH USB-to-Serial) |
| Serial IDs | `USB35INCHIPS` (Rev A), `USB35INCHIPSV2` (Rev A V2) |
| Color format | RGB565 little-endian (16 bpp) |
| OS view | Serial/COM port — NOT a monitor |

The screen is NOT seen as a display by the OS. All rendering is done by the host and sent as a bitmap over serial.

---

## Protocol

### 6-Byte Command Packet

```
Byte[0] = (x >> 2)
Byte[1] = ((x & 3) << 6) + (y >> 4)
Byte[2] = ((y & 15) << 4) + (ex >> 6)
Byte[3] = ((ex & 63) << 2) + (ey >> 8)
Byte[4] = (ey & 255)
Byte[5] = command_id
```

### Command Reference (Rev A / 3.5")

| Command | ID | Notes |
|---------|-----|-------|
| RESET | 101 | Device re-enumerates; wait 5s |
| CLEAR | 102 | Fill with white |
| SCREEN_OFF | 108 | Backlight off |
| SCREEN_ON | 109 | Backlight on |
| SET_BRIGHTNESS | 110 | 0=brightest, 255=darkest (inverted!) |
| SET_ORIENTATION | 121 | 0=portrait, 2=landscape |
| DISPLAY_BITMAP | 197 | Send RGB565 image chunk |
| HELLO | 69 | Query screen model/revision |

### Initialization Sequence

1. Open serial at 115200 baud, 8N1
2. Send `RESET` (101) — wait 5s for device re-enumeration
3. Send `HELLO` (69) to detect screen model
4. Send `SET_ORIENTATION` (121) to set rotation
5. Ready to send content

### Brightness

Scale is **inverted**: `level_abs = 255 - (brightness_pct * 255)`

```
  0% → 255 (brightest)
 50% → 127
100% →   0 (darkest/off)
```

---

## Project Structure

```
/home/evndr/dev/screen/
├── readme.md                          # User-facing documentation
├── agents.md                          # This file — for AI agents
├── turing-smart-screen.md             # Hardware/protocol reference
└── go/
    ├── go.mod / go.sum
    ├── bin/
    │   ├── dashboard                  # Main binary
    │   └── testrender                 # Single-frame debug binary
    ├── cmd/
    │   ├── dashboard/
    │   │   ├── main.go                # Stats loop + display
    │   │   └── api.go                 # HTTP message API
    │   └── testrender/
    │       └── main.go                # Single-frame render test
    ├── screen/
    │   ├── comm.go                    # Serial protocol (LcdComm)
    │   └── auto.go                    # Auto-detect serial port
    ├── render/
    │   ├── dashboard.go               # RGBA framebuffer + drawing + text wrapping
    │   └── messages.go                # Message queue + panel rendering
    └── monitor/
        ├── stats.go                   # ReadStats() — Linux stats aggregator
        ├── nvidia.go                  # nvidia-smi wrapper for GPU metrics
        └── utils.go                   # FormatBytes, FormatNetworkRate
```

---

## Build & Run

### Build

```bash
cd /home/evndr/dev/screen/go
go build -o bin/dashboard ./cmd/dashboard/
go build -o bin/testrender ./cmd/testrender/
```

### Physical screen

```bash
./bin/dashboard --port=/dev/ttyACM0 --refresh=3s
```

### Debug mode (no hardware — saves PNG frames to /tmp/)

```bash
./bin/dashboard -debug -refresh 2s
```

### Single-frame render test

```bash
go run ./cmd/testrender/
# Saves /tmp/dashboard_test_0.png (portrait) and /tmp/dashboard_test_2.png (landscape)
```

### Dashboard flags

```
-debug         Save PNG frames to /tmp/ instead of serial
-port string   Serial port or AUTO (default "AUTO")
-refresh       Update interval (default 2s)
-bg            Background image file (PNG/JPEG)
-message       Send a message text to the message panel
-message-file  Send message from a file
-message-ttl   Auto-dismiss message after N seconds (default 0 = never)
-message-color Message hex color, e.g. ff5500 (default ffffff)
-api-port      HTTP API port (0 to disable, default 8080)
-orientation   Screen orientation: portrait (default), landscape, reverse-portrait, reverse-landscape
```

---

## Stats Collected

| Metric | Source | Notes |
|--------|--------|-------|
| CPU % | `/proc/stat` | Overall usage |
| CPU Temp | `/sys/class/thermal/` or `/sys/class/hwmon/` | Celsius |
| RAM % | `/proc/meminfo` | MemAvailable-based |
| GPU % | `nvidia-smi` | Only if NVIDIA GPU present |
| GPU Temp | `nvidia-smi` | Celsius |
| GPU Mem | `nvidia-smi` | Used / Total MB |
| Disk % | `/proc/mounts` + `statvfs` | Real filesystems only (ext4, btrfs, xfs, f2fs) |
| Network RX/TX | `/proc/net/dev` | Cumulative bytes, rate formatted |

---

## Layout

**Portrait (default)** — 320×480, horizontal split at 56% height:
```
+----------------------------------+
|  System Info                     |
|  CPU / RAM / GPU / Disk / NET    |
|  y = 0 .. ~268                   |
+----------------------------------+
|  Messages                        |
|  (word-wrapped, colored borders) |
|  y = ~269 .. 479                 |
+----------------------------------+
```

**Landscape** — 480×320, vertical split at 50% width:
```
+------------------+------------------+
|  System Info     |  Messages        |
|  (left half)     |  (right half)    |
|  x = 0..239      |  x = 240..479    |
+------------------+------------------+
```

- Progress bars stretch to fill the info panel width (`barW = infoW - padX*2`)
- 10px vertical gap between system info sections (`render.SectionGap`)
- Message panel uses `DrawRightPanel()` which adapts to orientation (horizontal or vertical divider)
- Text wraps at word boundaries using `WrapText(text, maxWidth)`; only breaks words if they exceed the panel width

---

## Message API

Enabled with `--api-port 8080`:

```bash
POST /message  {"text": "Hello", "color": "ff5500", "ttl": 60}
GET  /messages
DELETE /message/:id
GET  /health
```

Message fields: `text` (max 200 chars), `color` (hex RGB, default `ffffff`), `ttl` (auto-dismiss seconds, default 0=never). Queue holds up to 20 messages (FIFO eviction). Long text is word-wrapped to fit the message panel width.

---

## Known Issues

1. **Stale processes:** Dashboard debug processes can accumulate. Kill with `kill -9 <PID>`.
2. **Brightness inversion:** Screen brightness scale is inverted (0=brightest, 255=darkest) — code handles this correctly.
3. **No GPU:** Machine has no NVIDIA GPU — GPU stats always zero/empty, handled gracefully.
4. **Background process output:** Running dashboard binaries in background mode with `terminal(background=true)` hangs after initial print. Use `go run ./cmd/testrender/` for debug frame verification.

---

## Key Implementation Details

- Serial port auto-detection: enumerates ports, prefers `ttyACM0`, falls back to first available
- RGB565 conversion: each pixel converted from RGBA 8-bit to RGB565 little-endian (5-6-5 bits)
- Framebuffer: 320×480 RGBA, dark background (`#0A0A1E`), Inconsolata 8x16 font
- Text wrapping: `WrapText()` measures glyph widths and splits at word boundaries
- PNG debugging: `render.Dashboard.SavePNG()` writes the RGBA framebuffer directly
- Signal handling: catches SIGINT/SIGTERM, turns screen off, exits cleanly
