package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"turing-screen/monitor"
	"turing-screen/render"
	"turing-screen/screen"
)

var (
	flagPort        = flag.String("port", "AUTO", "Serial port or AUTO")
	flagDebug       = flag.Bool("debug", false, "Save PNG frames to /tmp/ instead of sending to hardware")
	flagRefresh     = flag.Duration("refresh", 2*time.Second, "Update interval")
	flagBG          = flag.String("bg", "", "Background image file (PNG/JPEG)")
	flagMessage     = flag.String("message", "", "Send a message text to the right panel")
	flagMsgFile     = flag.String("message-file", "", "Send message from a file")
	flagAPIPort     = flag.Int("api-port", 8080, "HTTP API server port (0 to disable)")
	flagOrientation = flag.String("orientation", "portrait", "Screen orientation: portrait, landscape, reverse-portrait, reverse-landscape")
)

func main() {
	flag.Parse()

	// Parse orientation
	ori := parseOrientation(*flagOrientation)
	w, h := render.DashboardDimensions(ori)

	// Message queue setup
	msgQueue := render.NewMessageQueue()
	SetMessageQueue(msgQueue)

	// Send a message from CLI flag or message file
	if *flagMessage != "" {
		msgQueue.Add(*flagMessage, "ffffff", 0)
	}
	if *flagMsgFile != "" {
		data, err := os.ReadFile(*flagMsgFile)
		if err != nil {
			log.Fatalf("message-file: %v", err)
		}
		msgQueue.Add(string(data), "ffffff", 0)
	}

	// ── Background image ─────────────────────────────────────────
	var bg *image.RGBA
	if *flagBG != "" {
		var err error
		bg, err = render.LoadBackground(*flagBG, ori)
		if err != nil {
			log.Fatalf("background: %v", err)
		}
		log.Printf("[OK] Background: %s", *flagBG)
	}

	// ── API server ───────────────────────────────────────────────
	var srv *http.Server
	if *flagAPIPort > 0 {
		srv = StartAPI(msgQueue, *flagAPIPort)
	}

	// ── Screen setup ─────────────────────────────────────────────
	var lcd *screen.LcdComm

	if *flagDebug {
		fmt.Printf("[DEBUG] Simulated mode — %dx%d frames saved to /tmp/\n", w, h)
	} else {
		var err error
		lcd, err = screen.NewLcdComm(*flagPort)
		if err != nil {
			log.Fatalf("connect: %v", err)
		}
		defer lcd.Close()

		if err = lcd.Reset(); err != nil {
			log.Fatalf("reset: %v", err)
		}
		if err = lcd.InitializeComm(); err != nil {
			log.Fatalf("init: %v", err)
		}
		lcd.SetOrientation(byte(ori))
		lcd.SetBrightness(80)
		if err := lcd.Clear(); err != nil {
			log.Fatalf("clear: %v", err)
		}
		fmt.Printf("[OK] Screen initialized (%dx%d, orientation=%s)\n", w, h, *flagOrientation)
	}

	// ── Signal handling ──────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(*flagRefresh)
	defer ticker.Stop()

	// Track previous stats for network rate calculation
	var prev *monitor.Stats

	for {
		select {
		case <-quit:
			fmt.Println("\n[OK] Exiting")
			if lcd != nil {
				lcd.ScreenOff()
			}
			if srv != nil {
				srv.Close()
			}
			return

		case <-ticker.C:
			// Expire TTL messages
			msgQueue.Expire()

			stats, err := monitor.ReadStats(prev)
			if err != nil {
				log.Printf("[WARN] read stats: %v", err)
				continue
			}

			dash := render.NewDashboard(ori)
			if bg != nil {
				dash.SetBackground(bg)
			}
			dash.Clear(10, 10, 30) // dark background #0A0A1E, or composite bg image

			// ── Orientation-aware Layout ──────────────────────────────────────
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

			// Disks
			for _, d := range stats.Disks {
				dash.DrawText(labelX, y, d.Mount, 255, 100, 0)
				dash.DrawText(valueX, y, fmt.Sprintf("%.0f%%", d.Percent), 255, 255, 255)
				dash.DrawText(detailX, y, fmt.Sprintf("%s/%s",
					monitor.ShortBytes(d.Used), monitor.ShortBytes(d.Total)), 120, 120, 120)
				dash.DrawProgressBar(barX, y+10, barW, 10, d.Percent, 255, 100, 0)
				y += 44
			}

			// Network — RX in value column, TX in detail column
			if prev != nil {
				rx := monitor.ShortRate(stats, prev, false)
				tx := monitor.ShortRate(stats, prev, true)
				dash.DrawText(labelX, y, "NET", 50, 200, 100)
				dash.DrawText(valueX, y, "\u2193"+rx, 50, 255, 150)
				dash.DrawText(detailX, y, "\u2191"+tx, 50, 150, 255)
				y += 28
			}

			// ── Message panel ───────────────────────────────────────
			msgQueue.DrawRightPanel(dash)

			// ── Borders ──────────────────────────────────────────
			dash.DrawBorder(0, 0, w, h, 100, 100, 120)
			if isPortrait {
				dash.DrawBorder(0, 0, infoW, infoH, 80, 80, 100)
				dash.DrawBorder(0, infoH, w, h-infoH, 80, 80, 100)
			} else {
				dash.DrawBorder(0, 0, infoW, h, 80, 80, 100)
				dash.DrawBorder(infoW, 0, w-infoW, h, 80, 80, 100)
			}

			// Output
			if *flagDebug {
				dash.SavePNG(fmt.Sprintf("/tmp/dash_%d.png", time.Now().UnixNano()))
			} else {
				rgb565 := dash.ToRGB565()
				pw, ph := dash.PhysicalSize()
				if err := lcd.DisplayRGB565(0, 0, pw, ph, rgb565); err != nil {
					log.Printf("[ERROR] display: %v", err)
				}
			}

			prev = stats
		}
	}
}

func parseOrientation(s string) render.Orientation {
	switch s {
	case "landscape":
		return render.OrientationLandscape
	case "reverse-portrait":
		return render.OrientationReversePortrait
	case "reverse-landscape":
		return render.OrientationReverseLandscape
	default:
		return render.OrientationPortrait
	}
}
