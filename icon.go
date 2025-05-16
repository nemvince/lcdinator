package main

import (
	"image"
	"image/color"
)

// DrawIcon draws a small monochrome icon at (x, y) using an 8x8 bitmap.
func DrawIcon(fb *image.Gray, x, y int, icon [8]byte) {
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			if (icon[row]>>(7-col))&1 == 1 {
				fb.SetGray(x+col, y+row, color.Gray{Y: 0})
			}
		}
	}
}
