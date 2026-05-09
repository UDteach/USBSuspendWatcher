package ui

import (
	"image"
	"image/color"

	"github.com/lxn/walk"
)

func buildIcon() (*walk.Icon, error) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	bg := color.RGBA{R: 32, G: 48, B: 66, A: 255}
	accent := color.RGBA{R: 60, G: 190, B: 180, A: 255}
	hot := color.RGBA{R: 255, G: 190, B: 82, A: 255}
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, bg)
		}
	}
	for y := 7; y < 25; y++ {
		for x := 13; x < 19; x++ {
			img.Set(x, y, accent)
		}
	}
	for y := 20; y < 26; y++ {
		for x := 8; x < 24; x++ {
			img.Set(x, y, accent)
		}
	}
	for y := 4; y < 9; y++ {
		for x := 11; x < 14; x++ {
			img.Set(x, y, hot)
		}
		for x := 18; x < 21; x++ {
			img.Set(x, y, hot)
		}
	}
	for y := 11; y < 14; y++ {
		for x := 20; x < 26; x++ {
			img.Set(x, y, hot)
		}
	}
	return walk.NewIconFromImageForDPI(img, 96)
}
