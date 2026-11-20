package main

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"
)

func TestTrayIconIsWhite(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(trayIcon()))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	opaque := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A == 0 {
				continue
			}
			opaque++
			if c.R != 255 || c.G != 255 || c.B != 255 {
				t.Fatalf("pixel at %d,%d is not white: %v", x, y, c)
			}
		}
	}
	if opaque == 0 {
		t.Fatal("logo has no visible pixels")
	}
}
