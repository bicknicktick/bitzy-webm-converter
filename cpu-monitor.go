package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// CPUMonitor tracks CPU usage
type CPUMonitor struct {
	lastStats CPUStats
}

type CPUStats struct {
	user   uint64
	nice   uint64
	system uint64
	idle   uint64
	iowait uint64
	total  uint64
}

// NewCPUMonitor creates a new CPU monitor
func NewCPUMonitor() *CPUMonitor {
	monitor := &CPUMonitor{}
	monitor.lastStats = getCPUStats()
	return monitor
}

// GetCPUUsage returns current CPU usage percentage
func (m *CPUMonitor) GetCPUUsage() float64 {
	currentStats := getCPUStats()
	
	idleDiff := float64(currentStats.idle - m.lastStats.idle)
	totalDiff := float64(currentStats.total - m.lastStats.total)
	
	if totalDiff == 0 {
		return 0
	}
	
	usage := 100.0 * (1.0 - (idleDiff / totalDiff))
	m.lastStats = currentStats
	
	return usage
}

// ShouldThrottle returns true if CPU usage exceeds threshold
func (m *CPUMonitor) ShouldThrottle(threshold float64) bool {
	return m.GetCPUUsage() > threshold
}

func getCPUStats() CPUStats {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return CPUStats{}
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 8 {
				return CPUStats{}
			}
			
			user, _ := strconv.ParseUint(fields[1], 10, 64)
			nice, _ := strconv.ParseUint(fields[2], 10, 64)
			system, _ := strconv.ParseUint(fields[3], 10, 64)
			idle, _ := strconv.ParseUint(fields[4], 10, 64)
			iowait, _ := strconv.ParseUint(fields[5], 10, 64)
			
			total := user + nice + system + idle + iowait
			
			return CPUStats{
				user:   user,
				nice:   nice,
				system: system,
				idle:   idle,
				iowait: iowait,
				total:  total,
			}
		}
	}
	
	return CPUStats{}
}

// MonitorAndLog continuously monitors CPU usage
func MonitorAndLog(interval time.Duration) {
	monitor := NewCPUMonitor()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for range ticker.C {
		usage := monitor.GetCPUUsage()
		if usage > MaxCPUUsage {
			fmt.Printf("⚠️ CPU usage high: %.1f%% (threshold: %d%%)\n", usage, MaxCPUUsage)
		}
	}
}
