package main

import (
	"fmt"
	"image"
	"sync/atomic"

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
var globalServiceIndex *int32
var globalServiceAction *int32 // 0 = none, 1 = stop, 2 = restart
var globalDialogResult *int32
var globalRequestedScreen *int32

type SystemInfoScreen struct{}
type AboutScreen struct{}
type MenuScreen struct{}
type NetworkInfoScreen struct{}
type ServiceManagerScreen struct{}

type Screen interface {
	Draw(fb *image.Gray)
	HandleKey(key byte) bool
}

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

func (s *ServiceManagerScreen) Draw(fb *image.Gray) {
	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst:  fb,
		Src:  image.Black,
		Face: face,
	}
	services := GetRunningServices()
	idx := 0
	if globalServiceIndex != nil {
		idx = int(*globalServiceIndex)
	}
	if len(services) == 0 {
		d.Dot = fixed.P(0, 16)
		d.DrawString("No services found")
		return
	}
	if idx < 0 || idx >= len(services) {
		idx = 0
	}
	// Draw menu of services
	for i, svc := range services {
		y := 12 + i*16
		if y > 60 {
			break
		} // fit max 4 on screen
		if i == idx {
			d.Dot = fixed.P(0, y)
			d.DrawString("> " + svc)
		} else {
			d.Dot = fixed.P(0, y)
			d.DrawString("  " + svc)
		}
	}
	// Draw action dialog if needed
	if globalServiceAction != nil && *globalServiceAction != 0 {
		action := ""
		if *globalServiceAction == 1 {
			action = "Stop"
		} else if *globalServiceAction == 2 {
			action = "Restart"
		}
		d.Dot = fixed.P(0, 60)
		d.DrawString(fmt.Sprintf("%s %s? (OK/ESC)", action, services[idx]))
	}
}

func (s *SystemInfoScreen) HandleKey(key byte) bool {
	// No custom key handling
	return false
}

func (s *AboutScreen) HandleKey(key byte) bool {
	// No custom key handling (handled globally for ESC)
	return false
}

func (s *MenuScreen) HandleKey(key byte) bool {
	changed := false
	if globalInDialog != nil && atomic.LoadInt32(globalInDialog) == 1 {
		switch key {
		case KEY_ENTER:
			if globalDialogResult != nil && globalInDialog != nil {
				atomic.StoreInt32(globalDialogResult, 1)
				atomic.StoreInt32(globalInDialog, 0)
				changed = true
			}
		case KEY_ESC:
			if globalDialogResult != nil && globalInDialog != nil {
				atomic.StoreInt32(globalDialogResult, 2)
				atomic.StoreInt32(globalInDialog, 0)
				changed = true
			}
		}
		return changed
	}
	switch key {
	case KEY_UP:
		if globalMenuIndex != nil && atomic.LoadInt32(globalMenuIndex) > 0 {
			atomic.AddInt32(globalMenuIndex, -1)
			changed = true
		}
	case KEY_DOWN:
		if globalMenuIndex != nil && atomic.LoadInt32(globalMenuIndex) < 1 {
			atomic.AddInt32(globalMenuIndex, 1)
			changed = true
		}
	case KEY_ENTER:
		if globalInDialog != nil && globalMenuIndex != nil && globalDialogType != nil {
			atomic.StoreInt32(globalInDialog, 1)
			if atomic.LoadInt32(globalMenuIndex) == 0 {
				atomic.StoreInt32(globalDialogType, 1)
			} else if atomic.LoadInt32(globalMenuIndex) == 1 {
				atomic.StoreInt32(globalDialogType, 2)
			}
			changed = true
		}
	case KEY_ESC:
		// Exit menu, go back to main screen
		atomic.StoreInt32(globalMenuIndex, 0)
		atomic.StoreInt32(globalInDialog, 0)
		atomic.StoreInt32(globalDialogType, 0)
		atomic.StoreInt32(globalDialogResult, 0)
		if globalRequestedScreen != nil {
			atomic.StoreInt32(globalRequestedScreen, 0)
		}
		changed = true
	}
	return changed
}

func (s *NetworkInfoScreen) HandleKey(key byte) bool {
	changed := false
	if globalNetIfIndex == nil {
		return false
	}
	ifaces, _ := GetNetworkInterfaces()
	if len(ifaces) == 0 {
		return false
	}
	switch key {
	case KEY_UP:
		if *globalNetIfIndex > 0 {
			atomic.AddInt32(globalNetIfIndex, -1)
		} else {
			atomic.StoreInt32(globalNetIfIndex, int32(len(ifaces)-1))
		}
		changed = true
	case KEY_DOWN:
		if int(*globalNetIfIndex) < len(ifaces)-1 {
			atomic.AddInt32(globalNetIfIndex, 1)
		} else {
			atomic.StoreInt32(globalNetIfIndex, 0)
		}
		changed = true
	}
	return changed
}

func (s *ServiceManagerScreen) HandleKey(key byte) bool {
	changed := false
	services := GetRunningServices()
	if len(services) == 0 {
		return false
	}
	idx := int(*globalServiceIndex)
	if idx < 0 || idx >= len(services) {
		idx = 0
	}
	if atomic.LoadInt32(globalServiceAction) != 0 {
		switch key {
		case KEY_ENTER:
			if atomic.LoadInt32(globalServiceAction) == 1 {
				go ServiceAction(services[idx], "stop")
			} else if atomic.LoadInt32(globalServiceAction) == 2 {
				go ServiceAction(services[idx], "restart")
			}
			atomic.StoreInt32(globalServiceAction, 0)
			changed = true
		case KEY_ESC:
			atomic.StoreInt32(globalServiceAction, 0)
			changed = true
		}
		return changed
	}
	switch key {
	case KEY_UP:
		if *globalServiceIndex > 0 {
			atomic.AddInt32(globalServiceIndex, -1)
			changed = true
		}
	case KEY_DOWN:
		if *globalServiceIndex < int32(len(services))-1 {
			atomic.AddInt32(globalServiceIndex, 1)
			changed = true
		}
	case KEY_LEFT:
		atomic.StoreInt32(globalServiceAction, 1)
		changed = true
	case KEY_RIGHT:
		atomic.StoreInt32(globalServiceAction, 2)
		changed = true
	}
	return changed
}
