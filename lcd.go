package main

import (
	"image"
	"image/color"
)

type Drawable interface {
	Draw(fb *image.Gray)
}

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
	imageBounds := d.Framebuffer.Bounds()
	for y := imageBounds.Min.Y; y < imageBounds.Max.Y; y++ {
		for x := imageBounds.Min.X; x < imageBounds.Max.X; x++ {
			d.Framebuffer.SetGray(x, y, color.Gray{Y: 255})
		}
	}
}

func (d *Display) DrawDrawable(dr Drawable) {
	dr.Draw(d.Framebuffer)
}

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
