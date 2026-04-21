package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/inconsolata"
	"golang.org/x/image/math/fixed"
)

// Orientation defines the screen rotation.
type Orientation int

const (
	OrientationPortrait          Orientation = 0
	OrientationReversePortrait   Orientation = 1
	OrientationLandscape         Orientation = 2
	OrientationReverseLandscape  Orientation = 3
)

// DashboardDimensions returns width and height for a given orientation.
func DashboardDimensions(o Orientation) (w, h int) {
	switch o {
	case OrientationLandscape, OrientationReverseLandscape:
		return 480, 320
	default:
		return 320, 480
	}
}

// Dashboard is a pixel frame buffer with orientation awareness.
type Dashboard struct {
	img         *image.RGBA
	face        font.Face
	Orientation Orientation
	W           int
	H           int

	// Optional static background image (loaded once, composited each frame)
	bgImg *image.RGBA
}

// NewDashboard creates a new dashboard with the given orientation.
func NewDashboard(orientation Orientation) *Dashboard {
	w, h := DashboardDimensions(orientation)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	face := inconsolata.Regular8x16
	return &Dashboard{img: img, face: face, Orientation: orientation, W: w, H: h}
}

// LoadBackground loads a PNG/JPEG image and scales it to the dashboard dimensions for a given orientation.
func LoadBackground(path string, orientation Orientation) (*image.RGBA, error) {
	w, h := DashboardDimensions(orientation)

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open background image: %w", err)
	}
	defer f.Close()

	rawImg, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode background image: %w", err)
	}

	// Scale to target dimensions using nearest-neighbor
	scaled := scaleImage(rawImg, w, h)

	// Convert to RGBA
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(rgba, rgba.Bounds(), scaled, image.ZP, draw.Src)
	return rgba, nil
}

// scaleImage rescales src to target dimensions using nearest-neighbor.
func scaleImage(src image.Image, width, height int) *image.RGBA {
	srcBounds := src.Bounds()
	sw := srcBounds.Dx()
	sh := srcBounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			sx := dx * sw / width
			sy := dy * sh / height
			dst.Set(dx, dy, src.At(srcBounds.Min.X+sx, srcBounds.Min.Y+sy))
		}
	}
	return dst
}

// SetBackground sets a static background image. Call at startup.
func (d *Dashboard) SetBackground(bg *image.RGBA) {
	d.bgImg = bg
}

// Clear resets the framebuffer, compositing the background image if set.
func (d *Dashboard) Clear(r, g, b uint8) {
	if d.bgImg != nil {
		draw.Draw(d.img, d.img.Bounds(), d.bgImg, image.ZP, draw.Src)
	} else {
		draw.Draw(d.img, d.img.Bounds(), image.NewUniform(color.RGBA{R: r, G: g, B: b, A: 255}), image.ZP, draw.Src)
	}
}

// DrawText renders text at pixel position (x, y).
func (d *Dashboard) DrawText(x, y int, text string, r, g, b uint8) {
	if d.face == nil {
		d.face = basicfont.Face7x13
	}
	pt := fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)}
	dresser := &font.Drawer{
		Dst:  d.img,
		Src:  image.NewUniform(color.RGBA{R: r, G: g, B: b, A: 255}),
		Face: d.face,
		Dot:  pt,
	}
	dresser.DrawString(text)
}

// DrawRect draws a filled rectangle.
func (d *Dashboard) DrawRect(x, y, w, h int, r, g, b uint8) {
	clr := image.NewUniform(color.RGBA{R: r, G: g, B: b, A: 255})
	rect := image.Rect(x, y, x+w, y+h)
	draw.Draw(d.img, rect, clr, image.ZP, draw.Src)
}

// DrawBorder draws a hollow rectangle border (1px thick).
func (d *Dashboard) DrawBorder(x, y, w, h int, r, g, b uint8) {
	clr := color.RGBA{R: r, G: g, B: b, A: 255}
	// Top and bottom
	for dx := 0; dx < w; dx++ {
		d.img.Set(x+dx, y, clr)
		d.img.Set(x+dx, y+h-1, clr)
	}
	// Left and right
	for dy := 0; dy < h; dy++ {
		d.img.Set(x, y+dy, clr)
		d.img.Set(x+w-1, y+dy, clr)
	}
}

// DrawProgressBar draws a horizontal bar with border.
func (d *Dashboard) DrawProgressBar(x, y, w, h int, percent float64, r, g, b uint8) {
	// Background (brighter so bar outline is visible against dark bg)
	d.DrawRect(x, y, w, h, 70, 70, 90)

	// Fill (minimum 3px if non-zero so tiny percentages are clearly visible)
	fillW := int(float64(w) * percent / 100)
	if fillW < 3 && percent > 0 {
		fillW = 3
	}
	if fillW > w {
		fillW = w
	}
	d.DrawRect(x, y, fillW, h, r, g, b)

	// Border — 1px top, bottom, left, right
	border := color.RGBA{R: 100, G: 100, B: 100, A: 255}
	for dx := 0; dx < w; dx++ {
		d.img.Set(x+dx, y, border)
		d.img.Set(x+dx, y+h-1, border)
	}
	for dy := 0; dy < h; dy++ {
		d.img.Set(x, y+dy, border)
		d.img.Set(x+w-1, y+dy, border)
	}
}

// RGBA returns the underlying *image.RGBA for RGB565 encoding.
func (d *Dashboard) RGBA() *image.RGBA {
	return d.img
}

// ToRGB565 converts the dashboard image to RGB565 byte slice (little-endian),
// rotating into native 320x480 orientation if needed.
func (d *Dashboard) ToRGB565() []byte {
	phys := d.toPhysicalRGBA()
	bounds := phys.Bounds()
	buf := make([]byte, 0, bounds.Dx()*bounds.Dy()*2)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rr, gg, bb, _ := phys.At(x, y).RGBA()
			// RGBA returns 16-bit values: shift down to 8-bit
			r8 := uint8(rr >> 8)
			g8 := uint8(gg >> 8)
			b8 := uint8(bb >> 8)

			rgb565 := ((uint16(r8) >> 3) << 11) |
				((uint16(g8) >> 2) << 5) |
				(uint16(b8) >> 3)

			buf = append(buf, byte(rgb565), byte(rgb565>>8))
		}
	}

	return buf
}

// toPhysicalRGBA returns the image in native 320x480 orientation.
// For portrait: returns as-is. For landscape: rotates 90° counter-clockwise.
func (d *Dashboard) toPhysicalRGBA() *image.RGBA {
	if d.Orientation == OrientationPortrait || d.Orientation == OrientationReversePortrait {
		return d.img
	}

	// Landscape: rotate 90° counter-clockwise from 480x320 → 320x480
	src := d.img
	srcBounds := src.Bounds()
	sw := srcBounds.Dx() // 480
	sh := srcBounds.Dy() // 320

	dst := image.NewRGBA(image.Rect(0, 0, 320, 480))

	// dst(x, y) = src(y, 479-x) for 90° CCW rotation
	for sy := 0; sy < sh; sy++ {
		for sx := 0; sx < sw; sx++ {
			dx := sy
			dy := (sw - 1) - sx
			dst.Set(dx, dy, src.At(srcBounds.Min.X+sx, srcBounds.Min.Y+sy))
		}
	}

	return dst
}

// PhysicalSize returns the native width and height (320x480) that the hardware expects.
func (d *Dashboard) PhysicalSize() (w, h int) {
	return 320, 480
}

// SavePNG saves the dashboard as a PNG for debugging.
func (d *Dashboard) SavePNG(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, d.img)
}

// SectionGap is the vertical gap between system info sections on the left panel.
const SectionGap = 10

// PanelPaddingY is the vertical padding for both left and right panels.
const PanelPaddingY = 20

// PanelPaddingX is the horizontal padding within both left and right panels.
const PanelPaddingX = 8

// ── Message panel (right half for portrait, bottom half for landscape) ──────────────────────────────────

// MsgLineH is the height of each message line.
const MsgLineH = 18

// MsgStartY pads messages from the top of the message panel area (matches PanelPaddingY).
const MsgStartY = PanelPaddingY

// MessagePanelBounds returns the (x, y, w, h) of the message panel area for the current orientation.
// Portrait: bottom panel (top/bottom split at 56% height)
// Landscape: right panel (left/right split at 50% width)
func (d *Dashboard) MessagePanelBounds() (x, y, w, h int) {
	switch d.Orientation {
	case OrientationPortrait, OrientationReversePortrait:
		splitY := int(float64(d.H) * 0.56)
		return 0, splitY, d.W, d.H - splitY
	default:
		splitX := d.W / 2
		return splitX, 0, d.W - splitX, d.H
	}
}

// DrawDivider draws a vertical divider at x going from y0 to y1.
func (d *Dashboard) DrawDivider(x, y0, y1 int, r, g, b uint8) {
	for y := y0; y < y1; y++ {
		d.img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
	}
}

// DrawHorizontalDivider draws a horizontal divider from x0 to x1 at y.
func (d *Dashboard) DrawHorizontalDivider(y, x0, x1 int, r, g, b uint8) {
	for x := x0; x < x1; x++ {
		d.img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
	}
}

// DrawMessage draws a single message text at (panelX+4, y) with optional color,
// clipped to the message panel area.
func (d *Dashboard) DrawMessage(x, y int, text string, r, g, b uint8) {
	_, py, pw, ph := d.MessagePanelBounds()
	pt := fixed.Point26_6{X: fixed.I(x + 4), Y: fixed.I(y)}
	dresser := &font.Drawer{
		Dst:  &clipImage{d.img, x, py, x + pw, py + ph},
		Src:  image.NewUniform(color.RGBA{R: r, G: g, B: b, A: 255}),
		Face: d.face,
		Dot:  pt,
	}
	dresser.DrawString(text)
}

// clipImage wraps an RGBA image and clips drawing to a sub-rectangle.
type clipImage struct {
	*image.RGBA
	x0, y0, x1, y1 int
}

func (c *clipImage) Bounds() image.Rectangle {
	return image.Rect(c.x0, c.y0, c.x1, c.y1)
}

func (c *clipImage) At(x, y int) color.Color {
	if x < c.x0 || x >= c.x1 || y < c.y0 || y >= c.y1 {
		return color.RGBA{}
	}
	return c.RGBA.At(x, y)
}

func (c *clipImage) Set(x, y int, col color.Color) {
	if x < c.x0 || x >= c.x1 || y < c.y0 || y >= c.y1 {
		return
	}
	c.RGBA.Set(x, y, col)
}

// WrapText splits text into lines that fit within maxWidth pixels using the dashboard font.
// It wraps at word boundaries when possible, only breaking individual words if they are
// longer than the available width.
func (d *Dashboard) WrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	var lines []string
	var line string

	for _, word := range strings.Fields(text) {
		candidate := line
		if len(line) > 0 {
			candidate += " "
		}
		candidate += word

		if font.MeasureString(d.face, candidate).Ceil() > maxWidth {
			if len(line) == 0 {
				// Word itself is too long — force-break it
				var frag string
				for _, r := range word {
					test := frag + string(r)
					if font.MeasureString(d.face, test).Ceil() > maxWidth && len(frag) > 0 {
						lines = append(lines, frag)
						frag = string(r)
					} else {
						frag = test
					}
				}
				line = frag
			} else {
				lines = append(lines, line)
				line = word
			}
		} else {
			line = candidate
		}
	}
	if len(line) > 0 {
		lines = append(lines, line)
	}
	return lines
}

// DrawMessageBox draws a message with a colored left border and wrapped text lines,
// suitable for the right panel.  h is the height of each line.
func (d *Dashboard) DrawMessageBox(x, y, lineH int, lines []string, borderColor string, textR, textG, textB uint8) {
	if len(lines) == 0 {
		return
	}
	// Parse border color
	br, bg, bb := hexToRGB(borderColor)

	totalH := len(lines) * lineH
	// Left border bar spanning full message height
	d.DrawRect(x, y, 3, totalH, br, bg, bb)

	// Draw each line; baseline at y+(i+1)*lineH-4 centers the 16px text in its row
	for i, line := range lines {
		ly := y + (i+1)*lineH - 4
		d.DrawText(x+6, ly, line, textR, textG, textB)
	}
}

// hexToRGB converts a 6-char hex string (e.g. "ff5500") to R, G, B bytes.
func hexToRGB(hex string) (r, g, b uint8) {
	if len(hex) < 6 {
		return 200, 200, 200
	}
	fmt.Sscanf(hex[:6], "%02x%02x%02x", &r, &g, &b)
	return
}
