# Turing Smart Screen — Technical Overview

> How the 3.5" USB-C display works and how to build custom software for it.

---

## Hardware

| Property | Value |
|----------|-------|
| **MCU** | WCH CH552T 8-bit E8051 core |
| **Connection** | USB-C → virtual serial port (115200 baud, RTS/CTS) |
| **Display** | 3.5" IPS LCD, 320×480 pixels (portrait default) |
| **Color format** | RGB565 little-endian (16 bpp) |
| **USB VID:PID** | `1a86:5722` (WCH USB-to-Serial) |
| **Serial IDs** | `USB35INCHIPS` / `USB35INCHIPSV2` |
| **OS view** | Serial/COM port — NOT a monitor |

The screen is not seen as a display by the OS. All data is sent over a serial connection using a custom binary protocol.

---

## Protocol

### Command Packet Format

```
Byte[0] = (x >> 2)
Byte[1] = ((x & 3) << 6) + (y >> 4)
Byte[2] = ((y & 15) << 4) + (ex >> 6)
Byte[3] = ((ex & 63) << 2) + (ey >> 8)
Byte[4] = (ey & 255)
Byte[5] = command_id
```

### Command Reference (Rev A / 3.5")

| Command | ID | Description |
|---------|-----|-------------|
| RESET | 101 | Reset display |
| CLEAR | 102 | Clear to white |
| TO_BLACK | 103 | Screen to black (untested) |
| SCREEN_OFF | 108 | Turn LCD off |
| SCREEN_ON | 109 | Turn LCD on |
| SET_BRIGHTNESS | 110 | Set backlight (0=brightest, 255=darkest) |
| SET_ORIENTATION | 121 | Set rotation (see below) |
| DISPLAY_BITMAP | 197 | Send image data |
| DISPLAY_PIXELS | 195 | Draw line chart points |
| HELLO | 69 | Query screen model/revision |
| SET_MIRROR | 122 | Mirror rendering |

### Orientation Values

```python
PORTRAIT           = 0
REVERSE_PORTRAIT   = 1
LANDSCAPE          = 2
REVERSE_LANDSCAPE  = 3
```

### Brightness

Scale is inverted: `level_abs = 255 - (brightness_pct * 255)`

```
0%   → 255 (brightest)
50%  → 127
100% → 0   (darkest/off)
```

### Initialization Sequence

1. Open serial port at 115200 baud
2. Send `RESET` (101) — wait 5 seconds (COM port may re-enumerate)
3. Send `HELLO` (69) to detect screen model/revision
4. Send `SET_ORIENTATION` (121) to set rotation
5. Ready to send content

---

## Display Content

### Image Format

All content is rendered as a **bitmap** and sent to the screen:

- **Format**: RGB565 little-endian
- **Conversion**: PIL/Pillow renders text/shapes to an image, then converts each pixel to RGB565
- **Transfer**: Chunked over serial with `DISPLAY_BITMAP` command

### Content Types

| Type | Method |
|------|--------|
| Image/Background | `DisplayBitmap(path, x, y)` |
| Text | `DisplayText(text, x, y, font, size, color)` |
| Progress bar (linear) | `DisplayProgressBar(x, y, w, h, min, max, value)` |
| Progress bar (radial) | `DisplayRadialProgressBar(xc, yc, radius, ...)` |
| Line graph | `DisplayLineGraph(x, y, w, h, values)` |

### Transparent Text

Text with a transparent background requires a background image to be provided, which gets composited behind the text.

---

## Auto-Detection

```python
# VID/PID match
vid = 0x1a86, pid = 0x5722

# Serial number match
serial = "USB35INCHIPS"   # Rev A
serial = "USB35INCHIPSV2" # Rev A V2
```

---

## Building Custom Software

### Option 1 — Python Library (Recommended)

```python
from library.lcd.lcd_comm_rev_a import LcdCommRevA

lcd = LcdCommRevA(
    com_port="AUTO",
    display_width=320,
    display_height=480
)

lcd.Reset()
lcd.InitializeComm()
lcd.SetBrightness(50)

lcd.DisplayBitmap("background.png")
lcd.DisplayText("Hello!", x=10, y=10, font_size=24, font_color=(255, 255, 255))
lcd.DisplayProgressBar(x=10, y=50, width=200, height=20, value=65)

lcd.closeSerial()
```

Repository: https://github.com/mathoudebine/turing-smart-screen-python

### Option 2 — Raw Serial (C, Rust, etc.)

1. Enumerate serial ports, find `1a86:5722` or serial `USB35INCHIPS*`
2. Open at 115200 baud, 8N1, RTS/CTS on
3. Send initialization sequence above
4. Convert images to RGB565 little-endian
5. Send bitmap data in chunks with `DISPLAY_BITMAP` command
6. Loop: read sensors → update display at desired refresh rate

### Option 3 — Simulated Display Mode

The Python library supports a simulated LCD mode — develop and test themes without the physical screen.

---

## Supported Models

| Model | Size | Resolution |
|-------|------|------------|
| Turing Smart Screen | 3.5" | 320×480 |
| Turing Smart Screen | 5" | 480×800 |
| Turing Smart Screen | 8.8" | — |
| XuanFang 3.5" Rev B / Flagship | 3.5" | 320×480 |
| UsbPCMonitor 3.5" / 5" | 3.5" / 5" | — |
| Kipye Qiye Smart Display | 3.5" | 320×480 |

---

## Resources

- **Open-source project**: https://github.com/mathoudebine/turing-smart-screen-python
- **Hackaday article**: https://hackaday.com/2023/09/11/cheap-lcd-uses-usb-serial/
- **Protocol wiki**: https://github.com/mathoudebine/turing-smart-screen-python/wiki
- **CNX Software review**: https://www.cnx-software.com/2022/04/29/turing-smart-screen-a-low-cost-3-5-inch-usb-type-c-information-display/
- **Simulated LCD mode**: Develop themes without hardware
