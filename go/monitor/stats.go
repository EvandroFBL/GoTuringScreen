package monitor

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// DiskInfo holds stats for a single mounted filesystem.
type DiskInfo struct {
	Mount   string
	Name    string // e.g. nvme0n1p2
	Total   uint64
	Used    uint64
	Percent float64
}

// Stats holds the current system metrics.
type Stats struct {
	Timestamp time.Time

	// CPU
	CPUPercent   float64 // overall usage %
	CPUModel     string
	CPUFreqMHz  float64
	CPUCoreCount int

	// Memory
	MemTotal   uint64
	MemUsed    uint64
	MemPercent float64

	// GPU (NVIDIA only, 0 if unavailable)
	GPUName      string
	GPUPercent   float64
	GPUCoreTempC float64
	GPUMemUsedMB uint64
	GPUMemTotalMB uint64

	// Disks (multiple mounted filesystems)
	Disks []DiskInfo

	// Network cumulative bytes
	NetBytesSent   uint64
	NetBytesRecv   uint64

	// Temperature
	CPUTempC float64

	// System load
	Load1  float64
	Load5  float64
	Load15 float64
}

// ReadStats gathers all system metrics.
func ReadStats(prev *Stats) (*Stats, error) {
	s := &Stats{Timestamp: time.Now()}

	if runtime.GOOS == "linux" {
		readLinuxCPU(s)
		readLinuxMemory(s)
		readLinuxDisks(s)
		readLinuxNetwork(s, prev)
		readLinuxCPUTemp(s)
		readLinuxLoad(s)
		gpu, _ := readNvidiaGPU()
		if gpu != nil {
			s.GPUName = gpu.Name
			s.GPUPercent = gpu.UsedPercent
			s.GPUCoreTempC = gpu.TempC
			s.GPUMemUsedMB = gpu.MemUsedMB
			s.GPUMemTotalMB = gpu.MemTotalMB
		}
	}

	return s, nil
}

// --- Linux readers ---

func readLinuxCPU(s *Stats) {
	// /proc/cpuinfo
	data, _ := os.ReadFile("/proc/cpuinfo")
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				s.CPUModel = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "cpu MHz") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				if f, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
					s.CPUFreqMHz = f
				}
			}
		}
	}

	// /proc/stat for usage
	data, _ = os.ReadFile("/proc/stat")
	lines := strings.SplitN(string(data), "\n", 2)
	if len(lines) < 2 {
		return
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return
	}

	var user, nice, system, idle, iowait uint64
	fmt.Sscanf(strings.Join(fields[1:], " "), "%d %d %d %d %d", &user, &nice, &system, &idle, &iowait)

	total := user + nice + system + idle + iowait
	idleTime := idle + iowait

	if total > 0 {
		s.CPUPercent = float64(total-idleTime) / float64(total) * 100
	}

	s.CPUCoreCount = runtime.NumCPU()
}

func readLinuxMemory(s *Stats) {
	data, _ := os.ReadFile("/proc/meminfo")
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	memTotal, memAvailable := uint64(0), uint64(0)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			memTotal = parseMemLine(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			memAvailable = parseMemLine(line)
		}
	}

	s.MemTotal = memTotal * 1024
	s.MemUsed = (memTotal - memAvailable) * 1024
	if s.MemTotal > 0 {
		s.MemPercent = float64(s.MemUsed) / float64(s.MemTotal) * 100
	}
}

func parseMemLine(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		if v, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
			return v
		}
	}
	return 0
}

func readLinuxDisks(s *Stats) {
	// Read real block device sizes
	blockSizes := map[string]uint64{}
	for _, dev := range []string{"sda", "nvme0n1", "nvme0n1p1", "nvme0n1p2", "nvme0n1p3"} {
		if data, err := os.ReadFile("/sys/class/block/" + dev + "/size"); err == nil {
			if size, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil {
				blockSizes[dev] = size * 512
			}
		}
	}

	// Parse /proc/mounts for real disk mounts (skip tmpfs, dev, proc, etc.)
	data, _ := os.ReadFile("/proc/mounts")
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	type mountEntry struct{ device, mount, fsType string }
	var mounts []mountEntry
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 {
			device := fields[0]
			fsType := fields[2]
			mountPt := fields[1]
			// Only real disks
			if fsType == "ext4" || fsType == "btrfs" || fsType == "xfs" || fsType == "f2fs" {
				mounts = append(mounts, mountEntry{device, mountPt, fsType})
			}
		}
	}

	// Use statvfs for used/total per mount
	for _, m := range mounts {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(m.mount, &stat); err != nil {
			continue
		}
		total := stat.Blocks * uint64(stat.Bsize)
		used := (stat.Blocks - stat.Bfree) * uint64(stat.Bsize)
		pct := float64(0)
		if total > 0 {
			pct = float64(used) / float64(total) * 100
		}

		// Derive disk name from device (e.g. /dev/nvme0n1p2 -> nvme0n1p2)
		name := strings.TrimPrefix(m.device, "/dev/")
		// Use full block device total if available
		blockTotal := blockSizes[name]
		if blockTotal == 0 {
			blockTotal = total
		}

		s.Disks = append(s.Disks, DiskInfo{
			Mount:   m.mount,
			Name:    name,
			Total:   blockTotal,
			Used:    used,
			Percent: pct,
		})
	}
}

func readLinuxNetwork(s *Stats, prev *Stats) {
	data, _ := os.ReadFile("/proc/net/dev")
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var recv, sent uint64

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.ContainsAny(line, ":") {
			continue
		}
		fields := strings.FieldsFunc(line, func(r rune) bool {
			return r == ':' || (r >= 'a' && r <= 'z') || r == ' '
		})
		if len(fields) >= 10 {
			if v, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				recv += v
			}
			if v, err := strconv.ParseUint(fields[9], 10, 64); err == nil {
				sent += v
			}
		}
	}

	s.NetBytesRecv = recv
	s.NetBytesSent = sent
}

func readLinuxCPUTemp(s *Stats) {
	// Try thermal zone
	matches, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	for _, p := range matches {
		if data, err := os.ReadFile(p); err == nil {
			if temp, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 10); err == nil {
				s.CPUTempC = temp / 1000
				return
			}
		}
	}

	// Try hwmon
	matches2, _ := filepath.Glob("/sys/class/hwmon/hwmon*/temp*_input")
	for _, p := range matches2 {
		if data, err := os.ReadFile(p); err == nil {
			if temp, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 10); err == nil {
				s.CPUTempC = temp / 1000
				return
			}
		}
	}
}

func readLinuxLoad(s *Stats) {
	data, _ := os.ReadFile("/proc/loadavg")
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		s.Load1, _ = strconv.ParseFloat(fields[0], 64)
		s.Load5, _ = strconv.ParseFloat(fields[1], 64)
		s.Load15, _ = strconv.ParseFloat(fields[2], 64)
	}
}
