//go:build linux || windows

package screen

import (
	"fmt"
	"io"

	"go.bug.st/serial"
)

// Command IDs for Rev A displays (3.5" 320x480)
const (
	CmdReset          byte = 101
	CmdClear          byte = 102
	CmdScreenOff      byte = 108
	CmdScreenOn       byte = 109
	CmdSetBrightness  byte = 110
	CmdSetOrientation byte = 121
	CmdDisplayBitmap  byte = 197
	CmdHello          byte = 69
)

// Orientation values
const (
	OrientationPortrait         = 0
	OrientationReversePortrait   = 1
	OrientationLandscape        = 2
	OrientationReverseLandscape = 3
)

// Display dimensions for 3.5" Turing screen
const (
	DisplayWidth  = 320
	DisplayHeight = 480
)

// LcdComm handles serial communication with the Turing Smart Screen.
type LcdComm struct {
	port io.ReadWriteCloser
}

// NewLcdComm opens a serial connection to the display.
// Set port to "AUTO" to auto-detect, or specify a port like "/dev/ttyACM0" or "COM3".
func NewLcdComm(port string) (*LcdComm, error) {
	if port == "AUTO" {
		p, err := autoDetectPort()
		if err != nil {
			return nil, fmt.Errorf("auto-detect failed: %w", err)
		}
		port = p
	}

	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	p, err := serial.Open(port, mode)
	if err != nil {
		return nil, fmt.Errorf("open serial: %w", err)
	}

	return &LcdComm{port: p}, nil
}

// Close closes the serial connection.
func (l *LcdComm) Close() error {
	return l.port.Close()
}

// Reset resets the display and waits for it to re-enumerate.
func (l *LcdComm) Reset() error {
	return l.sendCommand(0, 0, 0, 0, CmdReset)
	// NOTE: In production, wait 5s for the device to re-enumerate.
	// A new serial port may appear after reset.
}

// InitializeComm queries the display ID and sets orientation.
func (l *LcdComm) InitializeComm() error {
	if err := l.sendCommand(0, 0, 0, 0, CmdHello); err != nil {
		return err
	}

	buf := make([]byte, 6)
	_, err := l.port.Read(buf)
	if err != nil {
		return fmt.Errorf("read hello response: %w", err)
	}

	return l.SetOrientation(OrientationPortrait)
}

// SetOrientation sets the screen rotation.
func (l *LcdComm) SetOrientation(orientation byte) error {
	return l.sendCommand(0, 0, 0, int(orientation), CmdSetOrientation)
}

// SetBrightness sets the backlight level (0=brightest, 255=darkest).
func (l *LcdComm) SetBrightness(level byte) error {
	return l.sendCommand(0, 0, 0, int(level), CmdSetBrightness)
}

// ScreenOff turns the display off.
func (l *LcdComm) ScreenOff() error {
	return l.sendCommand(0, 0, 0, 0, CmdScreenOff)
}

// ScreenOn turns the display on.
func (l *LcdComm) ScreenOn() error {
	return l.sendCommand(0, 0, 0, 0, CmdScreenOn)
}

// Clear fills the screen with white. Orientation must be set by the caller.
func (l *LcdComm) Clear() error {
	return l.sendCommand(0, 0, DisplayWidth-1, DisplayHeight-1, CmdClear)
}

// DisplayRGB565 sends an RGB565 bitmap to the display at the given coordinates.
func (l *LcdComm) DisplayRGB565(x, y, w, h int, rgb565 []byte) error {
	ex := x + w - 1
	ey := y + h - 1

	if err := l.sendCommand(x, y, ex, ey, CmdDisplayBitmap); err != nil {
		return err
	}

	const chunkSize = 4096
	for i := 0; i < len(rgb565); i += chunkSize {
		end := i + chunkSize
		if end > len(rgb565) {
			end = len(rgb565)
		}
		_, err := l.port.Write(rgb565[i:end])
		if err != nil {
			return fmt.Errorf("write image chunk: %w", err)
		}
	}

	return nil
}

// sendCommand builds and sends a 6-byte command packet.
func (l *LcdComm) sendCommand(x, y, ex, ey int, cmd byte) error {
	buf := []byte{
		byte(x >> 2),
		byte(((x & 3) << 6) + (y >> 4)),
		byte(((y & 15) << 4) + (ex >> 6)),
		byte(((ex & 63) << 2) + (ey >> 8)),
		byte(ey & 255),
		cmd,
	}
	_, err := l.port.Write(buf)
	return err
}
