package main

import (
	"fmt"
	"image"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var globalMenuIndex *int32
var globalInDialog *int32
var globalDialogType *int32
var globalNetIfIndex *int32

type SystemInfoScreen struct{}
type AboutScreen struct{}
type MenuScreen struct{}
type NetworkInfoScreen struct{}

func (s *SystemInfoScreen) Draw(fb *image.Gray) {
	DrawIcon(fb, 0, 2, IconCPU)
	DrawIcon(fb, 0, 18, IconRAM)
	DrawIcon(fb, 0, 34, IconDisk)
	DrawIcon(fb, 0, 50, IconClock)

	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst:  fb,
		Src:  image.Black,
		Face: face,
	}

	cpuUsage := GetCPUUsage()
	d.Dot = fixed.P(10, 12)
	d.DrawString(fmt.Sprintf("CPU: %2.0f%%", cpuUsage))

	memUsed, memTotal := GetMemInfo()
	d.Dot = fixed.P(10, 28)
	d.DrawString(fmt.Sprintf("RAM: %d/%d MB", memUsed, memTotal))

	diskUsed, diskTotal := GetDiskInfo()
	d.Dot = fixed.P(10, 44)
	d.DrawString(fmt.Sprintf("DSK: %d/%d GB", diskUsed, diskTotal))

	uptime := GetUptime()
	d.Dot = fixed.P(10, 60)
	d.DrawString(fmt.Sprintf("UPT: %s", uptime))
}

func (s *AboutScreen) Draw(fb *image.Gray) {
	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst:  fb,
		Src:  image.Black,
		Face: face,
	}
	d.Dot = fixed.P(0, 12)
	d.DrawString("LCDinator")
	d.Dot = fixed.P(0, 28)
	d.DrawString("by nemvince")
	d.Dot = fixed.P(0, 60)
	d.DrawString("version 1")
}

func (s *MenuScreen) Draw(fb *image.Gray) {
	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst:  fb,
		Src:  image.Black,
		Face: face,
	}
	menuItems := []string{"Shutdown", "Reboot"}
	for i, item := range menuItems {
		y := 16 + i*20
		if globalMenuIndex != nil && *globalMenuIndex == int32(i) {
			d.Dot = fixed.P(0, y)
			d.DrawString("> " + item)
		} else {
			d.Dot = fixed.P(0, y)
			d.DrawString("  " + item)
		}
	}
	// Draw confirmation dialog overlay if needed
	if globalInDialog != nil && *globalInDialog == 1 && globalDialogType != nil {
		msg := "Are you sure?"
		if *globalDialogType == 1 {
			msg = "Shutdown? (OK/ESC)"
		} else if *globalDialogType == 2 {
			msg = "Reboot? (OK/ESC)"
		}
		d.Dot = fixed.P(10, 55)
		d.DrawString(msg)
	}
}

func (s *NetworkInfoScreen) Draw(fb *image.Gray) {
	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst:  fb,
		Src:  image.Black,
		Face: face,
	}
	ifaces, _ := GetNetworkInterfaces()
	idx := 0
	if globalNetIfIndex != nil {
		idx = int(*globalNetIfIndex)
	}
	numIfaces := len(ifaces)
	if numIfaces == 0 {
		d.Dot = fixed.P(0, 16)
		d.DrawString("No interfaces")
		return
	}
	if idx < 0 || idx >= numIfaces {
		idx = 0
	}
	iface := ifaces[idx]
	// Name and status, with (idx/n)
	DrawIcon(fb, 0, 4, IconPlug)
	d.Dot = fixed.P(10, 12)
	d.DrawString(fmt.Sprintf("%s (%d/%d)", iface.Name, idx+1, numIfaces))
	// IP
	if iface.IP != "" {
		DrawIcon(fb, 0, 20, IconNet)
		d.Dot = fixed.P(10, 28)
		d.DrawString(fmt.Sprintf("IP: %s", iface.IP))
	} else {
		DrawIcon(fb, 0, 20, IconNetError)
		d.Dot = fixed.P(10, 28)
		status := "Down"
		if iface.Up {
			status = "Up"
		}
		d.DrawString(fmt.Sprintf("%s, no IP", status))
	}
	// Bandwidth (always show, even if 0)
	DrawIcon(fb, 0, 36, IconArrowUp)
	d.Dot = fixed.P(10, 44)
	d.DrawString(fmt.Sprintf("RX: %d KB/s", iface.RxRate/1024))
	DrawIcon(fb, 0, 52, IconArrowDown)
	d.Dot = fixed.P(10, 60)
	d.DrawString(fmt.Sprintf("TX: %d KB/s", iface.TxRate/1024))
}
