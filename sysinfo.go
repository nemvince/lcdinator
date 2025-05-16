package main

import (
	"fmt"
	gonet "net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	psnet "github.com/shirou/gopsutil/v3/net"
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

type NetInterfaceInfo struct {
	Name   string
	IP     string
	Up     bool
	RxRate int64 // bytes/sec
	TxRate int64 // bytes/sec
	Signal int   // dBm, -1 if not wireless
}

// For bandwidth calculation using gopsutil
var prevIOCounters = make(map[string][2]uint64)
var prevIOTimestamp = time.Now()

func GetNetworkInterfaces() ([]NetInterfaceInfo, error) {
	var result []NetInterfaceInfo
	ifaces, err := gonet.Interfaces()
	if err != nil {
		return nil, err
	}
	ioStats, _ := psnet.IOCounters(true)
	now := time.Now()
	dt := now.Sub(prevIOTimestamp).Seconds()
	prevIOTimestamp = now
	ioMap := make(map[string]psnet.IOCountersStat)
	for _, stat := range ioStats {
		ioMap[stat.Name] = stat
	}
	for _, iface := range ifaces {
		info := NetInterfaceInfo{Name: iface.Name, Up: iface.Flags&gonet.FlagUp != 0, RxRate: 0, TxRate: 0, Signal: -1}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*gonet.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				info.IP = ipnet.IP.String()
			}
		}
		if stat, ok := ioMap[iface.Name]; ok && dt > 0 {
			prev, ok := prevIOCounters[iface.Name]
			if ok {
				info.RxRate = int64(float64(stat.BytesRecv-prev[0]) / dt)
				info.TxRate = int64(float64(stat.BytesSent-prev[1]) / dt)
			}
			prevIOCounters[iface.Name] = [2]uint64{stat.BytesRecv, stat.BytesSent}
		}
		// Signal strength (unchanged)
		if _, err := os.Stat("/proc/net/wireless"); err == nil {
			wf, _ := os.Open("/proc/net/wireless")
			if wf != nil {
				defer wf.Close()
				var l string
				for i := 0; i < 2; i++ {
					fmt.Fscanln(wf, &l)
				}
				for {
					_, err := fmt.Fscanln(wf, &l)
					if err != nil {
						break
					}
					var wname string
					var sig int
					fmt.Sscanf(l, "%s %*d. %d.", &wname, &sig)
					wname = wname[:len(wname)-1]
					if wname == iface.Name {
						info.Signal = sig
					}
				}
			}
		}
		result = append(result, info)
	}
	return result, nil
}

func GetRunningServices() []string {
	cmd := exec.Command("systemctl", "list-units", "--type=service", "--state=running", "--no-legend", "--no-pager")
	out, err := cmd.Output()
	if err != nil {
		return []string{"systemctl error"}
	}
	lines := strings.Split(string(out), "\n")
	var result []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			result = append(result, fields[0])
		}
	}
	return result
}

func ServiceAction(service, action string) {
	cmd := exec.Command("systemctl", action, service)
	_ = cmd.Run()
}
