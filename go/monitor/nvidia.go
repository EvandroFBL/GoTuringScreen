package monitor

import (
	"os/exec"
	"strconv"
	"strings"
)

type gpuInfo struct {
	Name        string
	UsedPercent float64
	TempC       float64
	MemUsedMB   uint64
	MemTotalMB  uint64
}

// readNvidiaGPU queries nvidia-smi for GPU metrics.
// Returns nil if no NVIDIA GPU is available.
func readNvidiaGPU() (*gpuInfo, error) {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=name,temperature.gpu,utilization.gpu,memory.used,memory.total",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil, err
	}

	fields := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(fields) < 5 {
		return nil, nil
	}

	g := &gpuInfo{}
	g.Name = strings.TrimSpace(fields[0])
	g.TempC, _ = strconv.ParseFloat(strings.TrimSpace(fields[1]), 64)
	g.UsedPercent, _ = strconv.ParseFloat(strings.TrimSpace(fields[2]), 64)
	g.MemUsedMB, _ = strconv.ParseUint(strings.TrimSpace(fields[3]), 10, 64)
	g.MemTotalMB, _ = strconv.ParseUint(strings.TrimSpace(fields[4]), 10, 64)

	return g, nil
}
