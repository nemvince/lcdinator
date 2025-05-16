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
var globalServiceAction *int32     // 0 = none, 1 = stop, 2 = restart
var globalServiceViewOffset *int32 // Tracks the first visible service index
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
	numServices := len(services)

	selectedIndex := 0
	if globalServiceIndex != nil {
		selectedIndex = int(atomic.LoadInt32(globalServiceIndex))
	}

	viewOffset := 0
	if globalServiceViewOffset != nil { // Ensure it's initialized
		viewOffset = int(atomic.LoadInt32(globalServiceViewOffset))
	}

	if numServices == 0 {
		d.Dot = fixed.P(0, 16)
		d.DrawString("No services found")
		return
	}

	// Clamp selectedIndex (should be managed by HandleKey, but good for safety here too)
	if selectedIndex < 0 {
		selectedIndex = 0
	}
	if selectedIndex >= numServices {
		selectedIndex = numServices - 1
	}

	const maxItemsOnScreen = 3
	const itemHeight = 16
	const listStartY = 12 // Y position where the list of items starts

	// Adjust viewOffset to ensure it's valid and the selected item is visible.
	if numServices <= maxItemsOnScreen {
		viewOffset = 0
	} else {
		maxPossibleViewOffset := numServices - maxItemsOnScreen
		// Ensure selected item is visible
		if selectedIndex < viewOffset {
			viewOffset = selectedIndex
		} else if selectedIndex >= viewOffset+maxItemsOnScreen {
			viewOffset = selectedIndex - maxItemsOnScreen + 1
		}
		// Clamp viewOffset to its valid range
		if viewOffset < 0 {
			viewOffset = 0
		}
		if viewOffset > maxPossibleViewOffset {
			viewOffset = maxPossibleViewOffset
		}
	}
	if globalServiceViewOffset != nil { // Store the corrected/validated viewOffset
		atomic.StoreInt32(globalServiceViewOffset, int32(viewOffset))
	}

	// Draw menu of services
	for i := 0; i < maxItemsOnScreen; i++ {
		actualServiceIndex := viewOffset + i
		if actualServiceIndex >= numServices {
			break // No more services to draw
		}
		svc := services[actualServiceIndex]
		yPos := listStartY + i*itemHeight

		prefix := "  "
		if actualServiceIndex == selectedIndex {
			prefix = "> "
		}
		// Truncate service name if too long
		maxServiceNameLen := (fb.Bounds().Max.X / 7) - len(prefix) - 3 // Approx chars, 7px font, space for scrollbar
		if len(svc) > maxServiceNameLen && maxServiceNameLen > 0 {
			svc = svc[:maxServiceNameLen-1] + "…"
		}

		d.Dot = fixed.P(0, yPos)
		d.DrawString(prefix + svc)
	}

	// Draw scrollbar
	itemAreaHeight := maxItemsOnScreen * itemHeight
	scrollbarX := fb.Bounds().Max.X - 3 // Position for a 2px wide scrollbar

	if numServices > maxItemsOnScreen {
		thumbHeightRatio := float64(maxItemsOnScreen) / float64(numServices)
		thumbHeightPixels := int(thumbHeightRatio * float64(itemAreaHeight))
		if thumbHeightPixels < 3 { // Min thumb height
			thumbHeightPixels = 3
		}
		if thumbHeightPixels > itemAreaHeight {
			thumbHeightPixels = itemAreaHeight
		}

		scrollableRangePixels := itemAreaHeight - thumbHeightPixels
		totalScrollableItemPositions := numServices - maxItemsOnScreen

		var thumbTopRelativeY int
		if totalScrollableItemPositions > 0 {
			thumbTopRelativeY = (viewOffset * scrollableRangePixels) / totalScrollableItemPositions
		} else {
			thumbTopRelativeY = 0
		}
		thumbAbsoluteTopY := listStartY + thumbTopRelativeY

		for yOffset := 0; yOffset < thumbHeightPixels; yOffset++ {
			pixelY := thumbAbsoluteTopY + yOffset
			if pixelY >= listStartY && pixelY < listStartY+itemAreaHeight {
				fb.Set(scrollbarX, pixelY, image.Black)
				fb.Set(scrollbarX+1, pixelY, image.Black)
			}
		}
	}

	// Draw action dialog if needed
	if globalServiceAction != nil && atomic.LoadInt32(globalServiceAction) != 0 {
		action := ""
		currentAction := atomic.LoadInt32(globalServiceAction)
		if currentAction == 1 {
			action = "Stop"
		} else if currentAction == 2 {
			action = "Restart"
		}
		if selectedIndex >= 0 && selectedIndex < numServices {
			d.Dot = fixed.P(0, 60)
			d.DrawString(fmt.Sprintf("%s %s? (OK/ESC)", action, services[selectedIndex]))
		} else {
			atomic.StoreInt32(globalServiceAction, 0) // Clear action if index is bad
		}
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
	numServices := len(services)

	if globalServiceIndex == nil || globalServiceViewOffset == nil || globalServiceAction == nil {
		return false
	}

	selectedIndexForDialog := int(atomic.LoadInt32(globalServiceIndex))

	if numServices == 0 && atomic.LoadInt32(globalServiceAction) == 0 {
		return false
	}

	const maxItemsOnScreen = 3

	if atomic.LoadInt32(globalServiceAction) != 0 {
		switch key {
		case KEY_ENTER:
			if selectedIndexForDialog >= 0 && selectedIndexForDialog < numServices {
				actionType := atomic.LoadInt32(globalServiceAction)
				serviceToActOn := services[selectedIndexForDialog]
				if actionType == 1 { // Stop
					go ServiceAction(serviceToActOn, "stop")
				} else if actionType == 2 { // Restart
					go ServiceAction(serviceToActOn, "restart")
				}
			}
			atomic.StoreInt32(globalServiceAction, 0)
			changed = true
		case KEY_ESC:
			atomic.StoreInt32(globalServiceAction, 0)
			changed = true
		}
		return changed
	}

	currentSelectedIndex := int(atomic.LoadInt32(globalServiceIndex))
	currentViewOffset := int(atomic.LoadInt32(globalServiceViewOffset))

	switch key {
	case KEY_UP:
		if currentSelectedIndex > 0 {
			newSelectedIndex := currentSelectedIndex - 1
			atomic.StoreInt32(globalServiceIndex, int32(newSelectedIndex))
			if newSelectedIndex < currentViewOffset {
				atomic.StoreInt32(globalServiceViewOffset, int32(newSelectedIndex))
			}
			changed = true
		}
	case KEY_DOWN:
		if currentSelectedIndex < numServices-1 {
			newSelectedIndex := currentSelectedIndex + 1
			atomic.StoreInt32(globalServiceIndex, int32(newSelectedIndex))
			if newSelectedIndex >= currentViewOffset+maxItemsOnScreen {
				atomic.StoreInt32(globalServiceViewOffset, int32(newSelectedIndex-maxItemsOnScreen+1))
			}
			changed = true
		}
	case KEY_LEFT: // Trigger Stop action
		if numServices > 0 && currentSelectedIndex >= 0 && currentSelectedIndex < numServices {
			atomic.StoreInt32(globalServiceAction, 1) // 1 for Stop
			changed = true
		}
	case KEY_RIGHT: // Trigger Restart action
		if numServices > 0 && currentSelectedIndex >= 0 && currentSelectedIndex < numServices {
			atomic.StoreInt32(globalServiceAction, 2) // 2 for Restart
			changed = true
		}
	}

	// Final clamping and validation after potential changes
	if numServices == 0 {
		atomic.StoreInt32(globalServiceIndex, 0)
		atomic.StoreInt32(globalServiceViewOffset, 0)
	} else {
		// Clamp selectedIndex
		finalSelectedIndex := int(atomic.LoadInt32(globalServiceIndex))
		if finalSelectedIndex >= numServices {
			finalSelectedIndex = numServices - 1
		}
		if finalSelectedIndex < 0 {
			finalSelectedIndex = 0
		}
		atomic.StoreInt32(globalServiceIndex, int32(finalSelectedIndex))

		// Clamp viewOffset, ensuring selected item is visible
		finalViewOffset := int(atomic.LoadInt32(globalServiceViewOffset)) // Re-read
		if numServices <= maxItemsOnScreen {
			finalViewOffset = 0
		} else {
			maxPossibleViewOffset := numServices - maxItemsOnScreen
			// Ensure selected item is visible
			if finalSelectedIndex < finalViewOffset {
				finalViewOffset = finalSelectedIndex
			} else if finalSelectedIndex >= finalViewOffset+maxItemsOnScreen {
				finalViewOffset = finalSelectedIndex - maxItemsOnScreen + 1
			}
			// Clamp viewOffset itself
			if finalViewOffset < 0 {
				finalViewOffset = 0
			}
			if finalViewOffset > maxPossibleViewOffset {
				finalViewOffset = maxPossibleViewOffset
			}
		}
		atomic.StoreInt32(globalServiceViewOffset, int32(finalViewOffset))
	}
	return changed
}
