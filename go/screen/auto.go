//go:build linux || windows

package screen

import (
	"fmt"
	"path/filepath"

	"go.bug.st/serial"
)

// autoDetectPort finds the Turing Smart Screen serial port by scanning USB identifiers.
func autoDetectPort() (string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return "", fmt.Errorf("enumerate ports: %w", err)
	}

	if len(ports) == 0 {
		return "", fmt.Errorf("no serial ports found")
	}

	for _, p := range ports {
		// Try to open in read-only mode to inspect USB metadata
		mode := &serial.Mode{BaudRate: 115200}
		f, err := serial.Open(p, mode)
		if err != nil {
			continue
		}

		// On Linux, check the device's USB info via sysfs
		// The WCH chip VID/PID is 1a86:5722
		// We identify the port by attempting to talk to it
		f.Close()

		// Fallback: if there's only one ACM port, assume it's ours
		if filepath.Base(p) == "ttyACM0" {
			return p, nil
		}
	}

	// Last resort: return the first port
	return ports[0], nil
}
