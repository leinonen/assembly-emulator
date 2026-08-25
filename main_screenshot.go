package main

import (
	"image"
	"image/png"
	"os"

	emio "assembly-emulator/emulator/io"
	"assembly-emulator/machine"
)

// writeScreenshot renders the latest frame to a PNG file.
func writeScreenshot(m *machine.Machine, path string) error {
	var f emio.Frame
	m.Frame(&f)
	img := image.NewRGBA(image.Rect(0, 0, f.W, f.H))
	copy(img.Pix, f.Pix)
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	return png.Encode(out, img)
}
