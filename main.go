package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"os"
	"time"

	"go.bug.st/serial" // External dependency: go get go.bug.st/serial
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Copyright (c) 2021 arx.net - Thanos Chatziathanassiou . All rights reserved.
// This program is free software; you can redistribute it and/or
// modify it under the same terms as Perl itself.
// See [http://www.perl.com/perl/misc/Artistic.html](http://www.perl.com/perl/misc/Artistic.html)
//
// This Go program is a translation of the original Perl script.
// Based on work done by Saint-Frater on https://git.nox-rhea.org/globals/reverse-engineering/ezio-g500

// TODOs from original Perl script (still apply or are addressed):
// - make sure the serial port is in working condition without stty
//   (go.bug.st/serial usually handles this well on POSIX)
// - probably rewrite in C or Rust (This is a Go rewrite)
// - make the BMP parser works with stuff other than imagemagick
//   (This Go version also uses the hardcoded offset)

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

func renderTextToFramebuffer(text string) []byte {
	img := image.NewGray(image.Rect(0, 0, expectedImageWidth, expectedImageHeight))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst:  img,
		Src:  image.Black,
		Face: face,
		Dot:  fixed.P(0, face.Ascent),
	}
	d.DrawString(text)

	bytesPerScanline := expectedImageWidth / 8
	framebuffer := make([]byte, bytesPerScanline*expectedImageHeight)

	for y := 0; y < expectedImageHeight; y++ {
		for xByte := 0; xByte < bytesPerScanline; xByte++ {
			var b byte
			for bit := 0; bit < 8; bit++ {
				x := xByte*8 + bit
				if x >= expectedImageWidth {
					continue
				}
				if img.GrayAt(x, y).Y < 128 {
					b |= 1 << (7 - bit)
				}
			}
			framebuffer[y*bytesPerScanline+xByte] = b
		}
	}
	return framebuffer
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <text_to_render> [serial_device]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Default serial device: %s\n", defaultSerialDevice)
		os.Exit(1)
	}
	textToRender := os.Args[1]
	serialDevice := defaultSerialDevice
	if len(os.Args) > 2 {
		serialDevice = os.Args[2]
	}

	// Generate framebuffer from text
	bytesFromFile := renderTextToFramebuffer(textToRender)

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

	writeSerial(initCmd())
	time.Sleep(sleepDuration)
	writeSerial(homeCmd())
	time.Sleep(sleepDuration)
	writeSerial(clearCmd())
	time.Sleep(sleepDuration)

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
			writeSerial([]byte{cols[j] ^ 0xff})
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
			writeSerial([]byte{cols[j] ^ 0xff})
		}
	}

	fmt.Println("Image data sent to LCD.")
	time.Sleep(sleepDuration)
}
