package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"

	"github.com/egoist/mygo/ui"
)

//go:embed embed/logo.png
var logoPNG []byte

// icon wraps Lucide-style strokes. The panel tints them with the text color.
func icon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`))
}

var (
	iconRefresh  = icon(`<path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/><path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/><path d="M16 16h5v5"/>`)
	iconSettings = icon(`<line x1="4" x2="4" y1="21" y2="14"/><line x1="4" x2="4" y1="10" y2="3"/><line x1="12" x2="12" y1="21" y2="12"/><line x1="12" x2="12" y1="8" y2="3"/><line x1="20" x2="20" y1="21" y2="16"/><line x1="20" x2="20" y1="12" y2="3"/><line x1="2" x2="6" y1="14" y2="14"/><line x1="10" x2="14" y1="8" y2="8"/><line x1="18" x2="22" y1="16" y2="16"/>`)
	iconBack     = icon(`<path d="m15 18-6-6 6-6"/>`)
	iconChevron  = icon(`<path d="m6 9 6 6 6-6"/>`)
	iconAlert    = icon(`<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4"/><path d="M12 17h.01"/>`)
)

// trayIcon is the menu bar mark: the embedded logo, painted white and
// keeping its original transparency.
func trayIcon() []byte {
	src, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		return logoPNG
	}
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			dst.SetNRGBA(x-b.Min.X, y-b.Min.Y, color.NRGBA{R: 255, G: 255, B: 255, A: uint8(a >> 8)})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, dst)
	return buf.Bytes()
}
