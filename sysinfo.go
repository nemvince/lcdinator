package main

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

func GetCPUUsage() float64 {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0
	}
	defer f.Close()
	var user1, nice1, system1, idle1, iowait1, irq1, softirq1, steal1 uint64
	fmt.Fscanf(f, "cpu  %d %d %d %d %d %d %d %d", &user1, &nice1, &system1, &idle1, &iowait1, &irq1, &softirq1, &steal1)
	total1 := user1 + nice1 + system1 + idle1 + iowait1 + irq1 + softirq1 + steal1
	idleAll1 := idle1 + iowait1
	time.Sleep(100 * time.Millisecond)
	f.Seek(0, 0)
	fmt.Fscanf(f, "cpu  %d %d %d %d %d %d %d %d", &user1, &nice1, &system1, &idle1, &iowait1, &irq1, &softirq1, &steal1)
	total2 := user1 + nice1 + system1 + idle1 + iowait1 + irq1 + softirq1 + steal1
	idleAll2 := idle1 + iowait1
	deltaTotal := float64(total2 - total1)
	deltaIdle := float64(idleAll2 - idleAll1)
	if deltaTotal == 0 {
		return 0
	}
	return 100.0 * (1.0 - deltaIdle/deltaTotal)
}

func GetMemInfo() (used, total int) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	var memTotal, memFree, buffers, cached int
	scan := func() {
		var label string
		var value int
		for {
			_, err := fmt.Fscanf(f, "%s %d kB\n", &label, &value)
			if err != nil {
				break
			}
			switch label {
			case "MemTotal:":
				memTotal = value
			case "MemFree:":
				memFree = value
			case "Buffers:":
				buffers = value
			case "Cached:":
				cached = value
			}
		}
	}
	scan()
	total = memTotal / 1024
	used = (memTotal - memFree - buffers - cached) / 1024
	return
}

func GetDiskInfo() (used, total int) {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0, 0
	}
	total = int((stat.Blocks * uint64(stat.Bsize)) / (1024 * 1024 * 1024))
	used = int(((stat.Blocks - stat.Bfree) * uint64(stat.Bsize)) / (1024 * 1024 * 1024))
	return
}

func GetUptime() string {
	f, err := os.Open("/proc/uptime")
	if err != nil {
		return "?"
	}
	defer f.Close()
	var uptimeSeconds float64
	fmt.Fscanf(f, "%f", &uptimeSeconds)
	days := int(uptimeSeconds) / 86400
	hours := (int(uptimeSeconds) % 86400) / 3600
	minutes := (int(uptimeSeconds) % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %02dh %02dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%02dh %02dm", hours, minutes)
	}
	return fmt.Sprintf("%02dm", minutes)
}
