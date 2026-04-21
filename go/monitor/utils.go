package monitor

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// FormatBytes converts a byte count to a human-readable string.
func FormatBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	// thresholds: minimum n that requires the next unit up
	thresholds := []uint64{
		unit,         // n >= unit → display as KiB
		unit * unit, // n >= unit^2 → display as MiB
		unit << 20,  // GiB
		unit << 30,  // TiB
		unit << 40,  // PiB
		unit << 50,  // EiB
	}
	suffixes := "KMGTPE"

	exp := 0
	for exp < len(thresholds) && n >= thresholds[exp] {
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(thresholds[exp-1]), suffixes[exp-1])
}

// ShortBytes returns a compact byte string like "1.6G", "500M", "12K".
// Drops decimal fraction for values >= 10 to save horizontal space.
func ShortBytes(n uint64) string {
	s := FormatBytes(n)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "iB", "")
	// Drop decimal for integer part >= 10: "19.4G" -> "19G", "205.5G" -> "205G"
	// Keep decimal for single-digit integer part: "1.6G" -> "1.6G"
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			// Check if the digit before '.' is the only digit (position 0)
			if i == 1 && s[0] >= '0' && s[0] <= '9' {
				// Single digit before decimal, keep it
				break
			}
			// Drop the decimal part: find unit letter after digits
			j := i + 1
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			s = s[:i] + s[j:]
			break
		}
	}
	return s
}

// ShortRate returns a compact network rate like "1.6G", "500M", "12K".
func ShortRate(cur, prev *Stats, sent bool) string {
	s := FormatNetworkRate(cur, prev, sent)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "iB/s", "")
	s = strings.ReplaceAll(s, "B/s", "")
	return s
}
func FormatNetworkRate(cur, prev *Stats, sent bool) string {
	if prev == nil || cur == nil {
		return "0 B/s"
	}
	dt := cur.Timestamp.Sub(prev.Timestamp).Seconds()
	if dt <= 0 {
		return "0 B/s"
	}

	var bytes uint64
	if sent {
		bytes = cur.NetBytesSent - prev.NetBytesSent
	} else {
		bytes = cur.NetBytesRecv - prev.NetBytesRecv
	}

	rate := float64(bytes) / dt
	return FormatBytes(uint64(rate)) + "/s"
}

// FormatUptime converts a duration to a readable uptime string.
func FormatUptime(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	s := d - m*time.Minute
	if h > 24 {
		return fmt.Sprintf("%.0fd %02d:%02d:%02d", float64(h)/24, h%24, m, s)
	}
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// Round rounds a float to the given number of decimal places.
func Round(f float64, places int) float64 {
	mult := math.Pow(10, float64(places))
	return math.Round(f*mult) / mult
}
