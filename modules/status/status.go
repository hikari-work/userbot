package status

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

var botStartTime = time.Now()

func init() {
	manager.Register(&manager.Module{
		Name:        "Status",
		Description: "Menampilkan status server dan penggunaan sumber daya",
		Commands:    []string{"status", "server"},
		OnlyOut:     true,
		Handler:     statusHandler,
	})
}

type cpuTime struct {
	user, nice, system, idle, iowait, irq, softirq, steal, guest, guestNice uint64
}

func getCPUTime() (cpuTime, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return cpuTime{}, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	var t cpuTime
	var cpu string
	_, err = fmt.Fscanf(file, "%s %d %d %d %d %d %d %d %d %d %d",
		&cpu, &t.user, &t.nice, &t.system, &t.idle, &t.iowait, &t.irq, &t.softirq, &t.steal, &t.guest, &t.guestNice)
	if err != nil {
		return cpuTime{}, err
	}
	return t, nil
}

func calculateCPUUsage(t1, t2 cpuTime) float64 {
	t1Total := t1.user + t1.nice + t1.system + t1.idle + t1.iowait + t1.irq + t1.softirq + t1.steal
	t2Total := t2.user + t2.nice + t2.system + t2.idle + t2.iowait + t2.irq + t2.softirq + t2.steal

	t1Idle := t1.idle + t1.iowait
	t2Idle := t2.idle + t2.iowait

	totalDelta := float64(t2Total - t1Total)
	if totalDelta == 0 {
		return 0.0
	}
	idleDelta := float64(t2Idle - t1Idle)

	return (1.0 - idleDelta/totalDelta) * 100.0
}

func getSystemRAM() (total, used, available uint64, err error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	scanner := bufio.NewScanner(file)
	var memTotal, memFree, memAvailable, buffers, cached uint64
	for scanner.Scan() {
		line := scanner.Text()
		var val uint64
		if strings.HasPrefix(line, "MemTotal:") {
			_, err := fmt.Sscanf(line, "MemTotal: %d kB", &val)
			if err != nil {
				return 0, 0, 0, err
			}
			memTotal = val * 1024
		} else if strings.HasPrefix(line, "MemFree:") {
			_, err := fmt.Sscanf(line, "MemFree: %d kB", &val)
			if err != nil {
				return 0, 0, 0, err
			}
			memFree = val * 1024
		} else if strings.HasPrefix(line, "MemAvailable:") {
			_, err := fmt.Sscanf(line, "MemAvailable: %d kB", &val)
			if err != nil {
				return 0, 0, 0, err
			}
			memAvailable = val * 1024
		} else if strings.HasPrefix(line, "Buffers:") {
			_, err := fmt.Sscanf(line, "Buffers: %d kB", &val)
			if err != nil {
				return 0, 0, 0, err
			}
			buffers = val * 1024
		} else if strings.HasPrefix(line, "Cached:") {
			_, err := fmt.Sscanf(line, "Cached: %d kB", &val)
			if err != nil {
				return 0, 0, 0, err
			}
			cached = val * 1024
		}
	}

	if memTotal == 0 {
		return 0, 0, 0, fmt.Errorf("failed to parse MemTotal")
	}

	if memAvailable == 0 {
		memAvailable = memFree + buffers + cached
	}

	return memTotal, memTotal - memAvailable, memAvailable, nil
}

func getSystemUptime() (string, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "", err
	}
	parts := strings.Fields(string(data))
	if len(parts) == 0 {
		return "", fmt.Errorf("empty uptime file")
	}
	var seconds float64
	_, err = fmt.Sscanf(parts[0], "%f", &seconds)
	if err != nil {
		return "", err
	}

	duration := time.Duration(seconds) * time.Second
	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	duration -= minutes * time.Minute
	secs := duration / time.Second

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, secs), nil
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, secs), nil
	}
	return fmt.Sprintf("%dm %ds", minutes, secs), nil
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func statusHandler(ctx *ext.Context, update *ext.Update) error {
	uMsg := update.EffectiveMessage
	uChat := update.EffectiveChat()
	slog.Info("Handling status command")

	t1, err := getCPUTime()
	var cpuUsage float64
	if err == nil {
		time.Sleep(100 * time.Millisecond)
		t2, err2 := getCPUTime()
		if err2 == nil {
			cpuUsage = calculateCPUUsage(t1, t2)
		}
	}

	memTotal, memUsed, memAvail, err := getSystemRAM()
	ramStr := "N/A"
	if err == nil {
		ramPerc := (float64(memUsed) / float64(memTotal)) * 100.0
		ramStr = fmt.Sprintf("\n  • Total: <code>%s</code>\n  • Used: <code>%s (%.1f%%)</code>\n  • Free: <code>%s</code>",
			formatBytes(memTotal), formatBytes(memUsed), ramPerc, formatBytes(memAvail))
	}

	sysUptime, err := getSystemUptime()
	if err != nil {
		sysUptime = "N/A"
	}

	botUptimeRaw := time.Since(botStartTime).Round(time.Second)
	botUptime := botUptimeRaw.String()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	goroutines := runtime.NumGoroutine()
	goAlloc := formatBytes(m.Alloc)
	goSys := formatBytes(m.Sys)
	goHeapAlloc := formatBytes(m.HeapAlloc)
	goHeapSys := formatBytes(m.HeapSys)
	goHeapIdle := formatBytes(m.HeapIdle)
	gcCycles := m.NumGC

	// Format teks HTML
	htmlContent := fmt.Sprintf(
		"<b> STATUS SERVER &amp; USERBOT</b>\n"+
			"--------------------------------------------\n"+
			"<b> CPU Usage:</b> <code>%.2f%%</code>\n"+
			"<b> System RAM:</b>%s\n\n"+
			"<b> Go Runtime Stats:</b>\n"+
			"  • Goroutines: <code>%d</code>\n"+
			"  • Go Memory Alloc: <code>%s</code> (sys: <code>%s</code>)\n"+
			"  • Go Heap Alloc: <code>%s</code> (sys: <code>%s</code>)\n"+
			"  • Go Heap Idle: <code>%s</code>\n"+
			"  • Go GC Cycles: <code>%d</code>\n"+
			"--------------------------------------------\n"+
			"<b>⏰ Bot Uptime:</b> <code>%s</code>\n"+
			"<b>⚙️ Sys Uptime:</b> <code>%s</code>",
		cpuUsage, ramStr, goroutines, goAlloc, goSys, goHeapAlloc, goHeapSys, goHeapIdle, gcCycles, botUptime, sysUptime,
	)

	text, entities := utils.ParseHTML(htmlContent)
	_, err = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       uMsg.ID,
		Message:  text,
		Entities: entities,
	})

	return err
}
