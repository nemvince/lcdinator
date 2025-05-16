package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"go.bug.st/serial" // External dependency: go get go.bug.st/serial
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const defaultSerialDevice = "/dev/ttyS1" // Default for Checkpoint 12200 / P210 on Linux
const expectedImageWidth = 128
const expectedImageHeight = 64

// initCmd returns the init sequence for the LCD/printer
func initCmd() []byte {
	return []byte{0x1b, 0x40} // ESC @
}

// clearCmd returns the clear sequence for the LCD
func clearCmd() []byte {
	return []byte{0x0c} // FF (Form Feed)
}

// homeCmd returns the home sequence for the LCD
func homeCmd() []byte {
	return []byte{0x0b} // VT (Vertical Tab)
}

// findAddIdx calculates the bitmask and base index for transforming BMP row data to LCD column data.
func findAddIdx(scanlineNumTimes16 int) (addVal int, idxBase int) {
	scanlineGroup := scanlineNumTimes16 / (8 * 16)
	idxBase = scanlineGroup * expectedImageWidth
	posInScanlineGroup := (scanlineNumTimes16 % (8 * 16)) / 16
	addVal = 1 << uint(posInScanlineGroup)
	return
}

// Drawable is anything that can draw itself on a framebuffer.
type Drawable interface {
	Draw(fb *image.Gray)
}

// Display represents the LCD framebuffer and serial logic.
type Display struct {
	Width, Height int
	Framebuffer   *image.Gray
}

func NewDisplay(width, height int) *Display {
	return &Display{
		Width:       width,
		Height:      height,
		Framebuffer: image.NewGray(image.Rect(0, 0, width, height)),
	}
}

func (d *Display) Clear() {
	draw.Draw(d.Framebuffer, d.Framebuffer.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
}

func (d *Display) DrawDrawable(dr Drawable) {
	dr.Draw(d.Framebuffer)
}

// Packs the framebuffer into the display's expected byte format (monochrome, 1bpp, bottom-up)
func (d *Display) Pack() []byte {
	bytesPerScanline := d.Width / 8
	framebuffer := make([]byte, bytesPerScanline*d.Height)
	for y := 0; y < d.Height; y++ {
		flippedY := d.Height - 1 - y // vertical flip
		for xByte := 0; xByte < bytesPerScanline; xByte++ {
			var b byte
			for bit := 0; bit < 8; bit++ {
				x := xByte*8 + bit
				if x >= d.Width {
					continue
				}
				if d.Framebuffer.GrayAt(x, flippedY).Y < 128 {
					b |= 1 << (7 - bit)
				}
			}
			framebuffer[y*bytesPerScanline+xByte] = b
		}
	}
	return framebuffer
}

// DrawIcon draws a small monochrome icon at (x, y) using an 8x8 bitmap.
func DrawIcon(fb *image.Gray, x, y int, icon [8]byte) {
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			if (icon[row]>>(7-col))&1 == 1 {
				fb.SetGray(x+col, y+row, color.Gray{Y: 0}) // Use color.Gray for black
			}
		}
	}
}

// Example 8x8 icons (edit as needed)
var (
	IconCPU = [8]byte{
		0b00111100,
		0b01000010,
		0b10100101,
		0b10111101,
		0b10111101,
		0b10100101,
		0b01000010,
		0b00111100,
	}
	IconRAM = [8]byte{
		0b11111111,
		0b10011001,
		0b10111101,
		0b10111101,
		0b10111101,
		0b10111101,
		0b10011001,
		0b11111111,
	}
	IconDisk = [8]byte{
		0b00111100,
		0b01000010,
		0b10011001,
		0b10111101,
		0b10111101,
		0b10011001,
		0b01000010,
		0b00111100,
	}
	IconClock = [8]byte{
		0b00111100,
		0b01000010,
		0b10011001,
		0b10100101,
		0b10100001,
		0b10011001,
		0b01000010,
		0b00111100,
	}
)

// SystemInfoScreen draws system info and icons
// (uses /proc/meminfo, /proc/stat, and df for Linux)
type SystemInfoScreen struct{}

func (s *SystemInfoScreen) Draw(fb *image.Gray) {
	// Draw icons with reduced margins
	DrawIcon(fb, 0, 2, IconCPU)
	DrawIcon(fb, 0, 18, IconRAM)
	DrawIcon(fb, 0, 34, IconDisk)
	DrawIcon(fb, 0, 50, IconClock)

	// Draw labels and values
	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst:  fb,
		Src:  image.Black,
		Face: face,
	}

	// CPU usage
	cpuUsage := getCPUUsage()
	d.Dot = fixed.P(10, 12)
	d.DrawString(fmt.Sprintf("CPU: %2.0f%%", cpuUsage))

	// RAM usage
	memUsed, memTotal := getMemInfo()
	d.Dot = fixed.P(10, 28)
	d.DrawString(fmt.Sprintf("RAM: %d/%d MB", memUsed, memTotal))

	// Disk usage
	diskUsed, diskTotal := getDiskInfo()
	d.Dot = fixed.P(10, 44)
	d.DrawString(fmt.Sprintf("DSK: %d/%d GB", diskUsed, diskTotal))

	// Uptime
	uptime := getUptime()
	d.Dot = fixed.P(10, 60)
	d.DrawString(fmt.Sprintf("UPT: %s", uptime))
}

// getCPUUsage returns CPU usage percent (Linux, simple 100ms sample)
func getCPUUsage() float64 {
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

// getMemInfo returns used and total memory in MB (Linux)
func getMemInfo() (used, total int) {
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

// getDiskInfo returns used and total disk in GB (Linux, root fs)
func getDiskInfo() (used, total int) {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0, 0
	}
	total = int((stat.Blocks * uint64(stat.Bsize)) / (1024 * 1024 * 1024))
	used = int(((stat.Blocks - stat.Bfree) * uint64(stat.Bsize)) / (1024 * 1024 * 1024))
	return
}

// getUptime returns the system uptime as a human-readable string (Linux)
func getUptime() string {
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

// Example AboutScreen
// (You can add more screens as needed)
type AboutScreen struct{}

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

func main() {
	display := NewDisplay(expectedImageWidth, expectedImageHeight)
	serialDevice := defaultSerialDevice
	if len(os.Args) > 1 {
		serialDevice = os.Args[1]
	}

	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	port, err := serial.Open(serialDevice, mode)
	if err != nil {
		log.Fatalf("Cannot open serial port %s: %v\n", serialDevice, err)
	}
	defer port.Close()

	writeSerial := func(data []byte) {
		n, err := port.Write(data)
		if err != nil {
			log.Fatalf("Serial write error: %v", err)
		}
		if n < len(data) {
			log.Fatalf("Serial write error: wrote only %d of %d bytes", n, len(data))
		}
	}

	sleepDuration := 5 * time.Millisecond

	type screenDef struct {
		name string
		draw Drawable
	}
	// Add more screens here as needed
	screens := []screenDef{
		{"System Info", &SystemInfoScreen{}},
		{"About", &AboutScreen{}},
	}
	currentScreen := 0

	var requestedScreen int32 = 0
	redrawChan := make(chan struct{}, 1)

	// Goroutine to read keycodes from serial port and update screen index immediately
	go func() {
		buf := make([]byte, 1)
		for {
			port.SetReadTimeout(100 * time.Millisecond)
			n, _ := port.Read(buf)
			if n == 1 {
				log.Printf("Key pressed: 0x%02X", buf[0])
				var changed bool
				if buf[0] == 0x45 { // ok
					if atomic.LoadInt32(&requestedScreen) != 0 {
						atomic.StoreInt32(&requestedScreen, 0)
						changed = true
					}
				} else if buf[0] == 0x41 { // help
					if atomic.LoadInt32(&requestedScreen) != 1 {
						atomic.StoreInt32(&requestedScreen, 1)
						changed = true
					}
				}
				if changed {
					select {
					case redrawChan <- struct{}{}:
					default:
					}
				}
			}
		}
	}()

	firstIteration := true
	currentScreen = 0
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if firstIteration {
			writeSerial(initCmd())
			time.Sleep(sleepDuration)
			writeSerial(homeCmd())
			time.Sleep(sleepDuration)
			writeSerial(clearCmd())
			time.Sleep(sleepDuration)
			firstIteration = false
		}

		// Wait for either a redraw request or the next timer tick
		var doRedraw bool
		select {
		case <-redrawChan:
			doRedraw = true
		case <-ticker.C:
			doRedraw = true
		}

		if doRedraw {
			newScreen := int(atomic.LoadInt32(&requestedScreen))
			if newScreen >= 0 && newScreen < len(screens) {
				currentScreen = newScreen
			}

			display.Clear()
			display.DrawDrawable(screens[currentScreen].draw)
			bytesFromFile := display.Pack()

			bytesPerScanline := expectedImageWidth / 8
			expectedPixelDataSize := bytesPerScanline * expectedImageHeight

			if len(bytesFromFile) != expectedPixelDataSize {
				log.Printf("Warning: BMP pixel data size is %d bytes. Expected %d bytes for a %dx%d monochrome image.",
					len(bytesFromFile), expectedPixelDataSize, expectedImageWidth, expectedImageHeight)
				if len(bytesFromFile) < expectedPixelDataSize && len(bytesFromFile)%bytesPerScanline == 0 {
					padding := make([]byte, expectedPixelDataSize-len(bytesFromFile))
					bytesFromFile = append(bytesFromFile, padding...)
				} else if len(bytesFromFile) < expectedPixelDataSize {
					log.Fatalf("Pixel data significantly smaller than expected and not a multiple of scanline size. Aborting.")
				}
			}

			var reorderedScanlines []byte
			numScanlinesInFile := len(bytesFromFile) / bytesPerScanline
			for i := numScanlinesInFile - 1; i >= 0; i-- {
				start := i * bytesPerScanline
				end := start + bytesPerScanline
				reorderedScanlines = append(reorderedScanlines, bytesFromFile[start:end]...)
			}
			if len(reorderedScanlines) > expectedPixelDataSize {
				reorderedScanlines = reorderedScanlines[:expectedPixelDataSize]
			}

			cols := make([]byte, expectedPixelDataSize)
			for j := 0; j < bytesPerScanline; j++ {
				for k := 0; k < expectedImageHeight; k++ {
					scanlineBlockStartOffset := k * bytesPerScanline
					currentByteOffsetInSource := scanlineBlockStartOffset + j
					if currentByteOffsetInSource >= len(reorderedScanlines) {
						continue
					}
					currentByte := reorderedScanlines[currentByteOffsetInSource]
					add, idxBase := findAddIdx(scanlineBlockStartOffset)
					targetColBase := idxBase + (j * 8)
					if targetColBase+7 >= len(cols) {
						continue
					}
					if (currentByte & 0x80) != 0 {
						cols[targetColBase+0] += byte(add)
					}
					if (currentByte & 0x40) != 0 {
						cols[targetColBase+1] += byte(add)
					}
					if (currentByte & 0x20) != 0 {
						cols[targetColBase+2] += byte(add)
					}
					if (currentByte & 0x10) != 0 {
						cols[targetColBase+3] += byte(add)
					}
					if (currentByte & 0x08) != 0 {
						cols[targetColBase+4] += byte(add)
					}
					if (currentByte & 0x04) != 0 {
						cols[targetColBase+5] += byte(add)
					}
					if (currentByte & 0x02) != 0 {
						cols[targetColBase+6] += byte(add)
					}
					if (currentByte & 0x01) != 0 {
						cols[targetColBase+7] += byte(add)
					}
				}
			}

			writeSerial([]byte{0x1B, 0x47})
			time.Sleep(sleepDuration * 100)

			skipCounter := 0
			for i := 0; i < len(cols); i += 64 {
				skipCounter++
				if skipCounter%2 == 0 {
					continue
				}
				limit := i + 64
				if limit > len(cols) {
					limit = len(cols)
				}
				for j := i; j < limit; j++ {
					writeSerial([]byte{cols[j]})
				}
			}

			skipCounter = 0
			for i := 0; i < len(cols); i += 64 {
				skipCounter++
				if skipCounter%2 != 0 {
					continue
				}
				limit := i + 64
				if limit > len(cols) {
					limit = len(cols)
				}
				for j := i; j < limit; j++ {
					writeSerial([]byte{cols[j]})
				}
			}

			fmt.Printf("Screen: %s\n", screens[currentScreen].name)
		}
	}
}
