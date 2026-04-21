# Turing Smart Screen Dashboard

A Go dashboard for the 3.5" USB-C Turing Smart Screen (320×480, RGB565 serial).

Displays live system stats (CPU, RAM, GPU, Disk, Network) and a scrollable message queue with word-wrapped text.

**Location:** `/home/evndr/dev/screen/`
**Language:** Go 1.25+

---

## Requirements

- Linux (tested on 6.x)
- Go 1.25+
- Physical screen: USB-C connection to `/dev/ttyACM0` (or auto-detected)

---

## Build

```bash
cd /home/evndr/dev/screen/go
go build -o bin/dashboard ./cmd/dashboard/
go build -o bin/testrender  ./cmd/testrender/
```

---

## Run

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

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `AUTO` | Serial port or `AUTO` to detect |
| `--refresh` | `2s` | Stats update interval |
| `--bg` | _none_ | Background image (PNG/JPEG, scaled to fit) |
| `--message` | _none_ | Send a message text to the message panel |
| `--message-file` | _none_ | Send message from a file |
| `--api-port` | `8080` | HTTP API port (`0` to disable) |
| `--debug` | `false` | Save PNG frames to `/tmp/` instead of serial |
| `--orientation` | `portrait` | `portrait`, `landscape`, `reverse-portrait`, `reverse-landscape` |

---

## Orientation

| Mode | Resolution | Layout |
|------|------------|--------|
| **portrait** (default) | 320×480 | Info panel on top (56% height), messages on bottom |
| **landscape** | 480×320 | Info panel on left (50% width), messages on right |
| **reverse-portrait** | 320×480 | Upside-down portrait |
| **reverse-landscape** | 480×320 | Upside-down landscape |

```bash
./bin/dashboard -orientation landscape
```

Progress bars stretch to fill the info panel width automatically.

---

## Message API

Enabled with `--api-port 8080` (or any non-zero port).

```bash
# Send a message
curl -X POST http://localhost:8080/message \
  -H "Content-Type: application/json" \
  -d '{"text":"Hello","color":"ff5500","ttl":60}'

# List messages
curl http://localhost:8080/messages

# Delete a message
curl -X DELETE http://localhost:8080/message/<id>
```

Message fields:
- `text` — string (max 200 chars, word-wrapped to fit panel width)
- `color` — optional hex RGB, e.g. `ff5500` (default: `ffffff` white)
- `ttl` — optional auto-dismiss seconds (default: `0` = never)

Queue holds up to 20 messages (FIFO eviction). Long messages wrap at word boundaries across multiple lines.

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

---

## Stats

| Metric | Source |
|--------|--------|
| CPU % | `/proc/stat` |
| CPU Temp | `/sys/class/thermal/` or `/sys/class/hwmon/` |
| RAM % | `/proc/meminfo` |
| GPU % / Temp / Mem | `nvidia-smi` (if NVIDIA GPU present) |
| Disk % | `/proc/mounts` + `statvfs` (ext4, btrfs, xfs, f2fs) |
| Network RX/TX | `/proc/net/dev` |

---

## Background Image

```bash
./bin/dashboard -bg /path/to/image.png
```

- Image is scaled to the dashboard dimensions at startup (nearest-neighbor)
- Composited onto the framebuffer before each frame's content
- If omitted: solid dark background (`#0A0A1E`)

---

## Project Structure

```
/home/evndr/dev/screen/
├── readme.md
├── agents.md                          # Agent handbook
├── turing-smart-screen.md             # Hardware protocol reference
└── go/
    ├── go.mod / go.sum
    ├── bin/
    │   ├── dashboard                  # Main binary
    │   └── testrender                 # Debug render binary
    ├── cmd/
    │   ├── dashboard/
    │   │   ├── main.go                # Stats loop + display
    │   │   └── api.go                 # HTTP message API
    │   └── testrender/
    │       └── main.go                # Single-frame PNG debug
    ├── screen/
    │   ├── comm.go                    # Serial protocol (LcdComm)
    │   └── auto.go                    # Auto-detect serial port
    ├── render/
    │   ├── dashboard.go               # RGBA framebuffer + drawing + text wrapping
    │   └── messages.go                # Message queue + panel rendering
    └── monitor/
        ├── stats.go                   # Linux stats reader
        ├── nvidia.go                  # nvidia-smi wrapper
        └── utils.go                   # FormatBytes, network rate formatting
```
