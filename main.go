package main

import (
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"go.bug.st/serial"
)

const defaultSerialDevice = "/dev/ttyS1"
const expectedImageWidth = 128
const expectedImageHeight = 64

func findAddIdx(scanlineNumTimes16 int) (addVal int, idxBase int) {
	scanlineGroup := scanlineNumTimes16 / (8 * 16)
	idxBase = scanlineGroup * expectedImageWidth
	posInScanlineGroup := (scanlineNumTimes16 % (8 * 16)) / 16
	addVal = 1 << uint(posInScanlineGroup)
	return
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
	screens := []screenDef{
		{"System Info", &SystemInfoScreen{}},
		{"About", &AboutScreen{}},
	}
	currentScreen := 0

	var requestedScreen int32 = 0
	redrawChan := make(chan struct{}, 1)

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
			writeSerial([]byte{0x1b, 0x40})
			time.Sleep(sleepDuration)
			writeSerial([]byte{0x0b})
			time.Sleep(sleepDuration)
			writeSerial([]byte{0x0c})
			time.Sleep(sleepDuration)
			firstIteration = false
		}

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
			if firstIteration {
				time.Sleep(sleepDuration * 100)
			}

			// Write every other 64-byte block in two passes: odd-indexed first, then even-indexed.
			for pass := 0; pass < 2; pass++ {
				for i := 0; i < len(cols); i += 64 {
					if (i/64)%2 != pass {
						continue
					}
					limit := min(i+64, len(cols))
					writeSerial(cols[i:limit])
				}
			}
			fmt.Printf("Screen: %s\n", screens[currentScreen].name)
		}
	}
}
