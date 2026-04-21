package main

import (
	"fmt"
	"image/color"
	"os"

	"turing-screen/monitor"
	"turing-screen/render"
)

func main() {
	stats, err := monitor.ReadStats(nil)
	if err != nil {
		fmt.Printf("read stats error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("CPU: %.1f%% | RAM: %.1f%% | Temp: %.1fC | Disks: %d | Load: %.2f\n",
		stats.CPUPercent, stats.MemPercent, stats.CPUTempC, len(stats.Disks), stats.Load1)

	orientations := []render.Orientation{
		render.OrientationPortrait,
		render.OrientationLandscape,
	}

	for _, ori := range orientations {
		w, h := render.DashboardDimensions(ori)
		fmt.Printf("\nTesting orientation %v (%dx%d)\n", ori, w, h)

		dash := render.NewDashboard(ori)

		// Fill background black so white text is visible in PNG
		img := dash.RGBA()
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.Set(x, y, color.RGBA{0, 0, 0, 255})
			}
		}

		isPortrait := ori == render.OrientationPortrait || ori == render.OrientationReversePortrait

		var infoW, infoH int
		if isPortrait {
			infoW = w
			infoH = int(float64(h) * 0.56)
		} else {
			infoW = w / 2
			infoH = h
		}

		padX := render.PanelPaddingX
		barX := padX
		barW := infoW - padX*2
		labelX := padX
		valueX := labelX + 48
		detailX := infoW - padX - 72
		y := render.PanelPaddingY

		// CPU
		dash.DrawText(labelX, y, "CPU", 0, 200, 255)
		cpuPct := fmt.Sprintf("%.1f%%", stats.CPUPercent)
		if stats.CPUPercent >= 10 {
			cpuPct = fmt.Sprintf("%.0f%%", stats.CPUPercent)
		}
		dash.DrawText(valueX, y, cpuPct, 255, 255, 255)
		if stats.CPUTempC > 0 {
			dash.DrawText(detailX, y, fmt.Sprintf("%.0fC", stats.CPUTempC), 255, 100, 50)
		}
		dash.DrawProgressBar(barX, y+10, barW, 10, stats.CPUPercent, 0, 200, 255)
		y += 44

		// RAM
		dash.DrawText(labelX, y, "RAM", 180, 0, 255)
		ramPct := fmt.Sprintf("%.1f%%", stats.MemPercent)
		if stats.MemPercent >= 10 {
			ramPct = fmt.Sprintf("%.0f%%", stats.MemPercent)
		}
		dash.DrawText(valueX, y, ramPct, 255, 255, 255)
		dash.DrawText(detailX, y, fmt.Sprintf("%s/%s",
			monitor.ShortBytes(stats.MemUsed), monitor.ShortBytes(stats.MemTotal)), 120, 120, 120)
		dash.DrawProgressBar(barX, y+10, barW, 10, stats.MemPercent, 180, 0, 255)
		y += 44

		// GPU
		if stats.GPUName != "" {
			dash.DrawText(labelX, y, "GPU", 0, 255, 180)
			gpuPct := fmt.Sprintf("%.1f%%", stats.GPUPercent)
			if stats.GPUPercent >= 10 {
				gpuPct = fmt.Sprintf("%.0f%%", stats.GPUPercent)
			}
			dash.DrawText(valueX, y, gpuPct, 255, 255, 255)
			if stats.GPUCoreTempC > 0 {
				dash.DrawText(detailX, y, fmt.Sprintf("%.0fC", stats.GPUCoreTempC), 255, 180, 50)
			}
			dash.DrawProgressBar(barX, y+10, barW, 10, stats.GPUPercent, 0, 255, 180)
			y += 44
		}

		// NET (mock)
		dash.DrawText(labelX, y, "NET", 50, 200, 100)
		dash.DrawText(valueX, y, "\u21930", 50, 255, 150)
		dash.DrawText(detailX, y, "\u21910", 50, 150, 255)
		y += 28

		// Disks
		for _, d := range stats.Disks {
			dash.DrawText(labelX, y, d.Mount, 255, 100, 0)
			dash.DrawText(valueX, y, fmt.Sprintf("%.0f%%", d.Percent), 255, 255, 255)
			dash.DrawText(detailX, y, fmt.Sprintf("%s/%s",
				monitor.ShortBytes(d.Used), monitor.ShortBytes(d.Total)), 120, 120, 120)
			dash.DrawProgressBar(barX, y+10, barW, 10, d.Percent, 255, 100, 0)
			y += 44
		}

		// Messages
		msgQueue := render.NewMessageQueue()
		msgQueue.Add("Hello from dashboard!", "ff5500", 0)
		msgQueue.Add("Compact layout", "00ff88", 0)
		msgQueue.Add("Disk stats now", "88ccff", 0)
		msgQueue.DrawRightPanel(dash)

		// Borders
		dash.DrawBorder(0, 0, w, h, 100, 100, 120)
		if isPortrait {
			dash.DrawBorder(0, 0, infoW, infoH, 80, 80, 100)
			dash.DrawBorder(0, infoH, w, h-infoH, 80, 80, 100)
		} else {
			dash.DrawBorder(0, 0, infoW, h, 80, 80, 100)
			dash.DrawBorder(infoW, 0, w-infoW, h, 80, 80, 100)
		}

		path := fmt.Sprintf("/tmp/dashboard_test_%v.png", ori)
		if err := dash.SavePNG(path); err != nil {
			fmt.Printf("save png error: %v\n", err)
			continue
		}

		rgb := dash.ToRGB565()
		fmt.Printf("Dashboard: %dx%d, RGB565 size: %d bytes\n", w, h, len(rgb))
		fmt.Printf("PNG saved to %s\n", path)
	}

	fmt.Printf("OK\n")
}
