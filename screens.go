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

type SystemInfoScreen struct{}
type AboutScreen struct{}

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
