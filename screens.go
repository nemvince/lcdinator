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

type SystemInfoScreen struct{}
type AboutScreen struct{}
type MenuScreen struct{}

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
