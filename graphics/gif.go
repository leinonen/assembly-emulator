package graphics

import (
	"image"
	"image/color"
	"image/gif"
	"os"

	emio "assembly-emulator/emulator/io"
	"assembly-emulator/machine"
)

func ebitenBlack() color.RGBA { return color.RGBA{0, 0, 0, 255} }

// RecordGIF runs the machine deterministically by virtual frames and
// writes an animated GIF with the given number of captured frames. Every
// second display frame (70 Hz) is captured, giving ~35 fps playback.
func RecordGIF(m *machine.Machine, path string, frames int) error {
	m.RenderFrames = false
	out := &gif.GIF{LoopCount: 0}
	var f emio.Frame
	frameCycles := m.Opts.CPUHz / 70
	for i := 0; i < frames && !m.Exited(); i++ {
		// Advance two display frames.
		if err := m.RunCycles(frameCycles * 2); err != nil {
			return err
		}
		m.ForceFrame()
		m.Frame(&f)
		img, pal := quantize(&f)
		_ = pal
		out.Image = append(out.Image, img)
		out.Delay = append(out.Delay, 3) // 30 ms
	}
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	return gif.EncodeAll(fh, out)
}

// quantize converts an RGBA frame into a paletted image, building the
// palette from the distinct colours present (at most 256; extra colours
// map to the nearest existing entry).
func quantize(f *emio.Frame) (*image.Paletted, color.Palette) {
	index := map[uint32]uint8{}
	var pal color.Palette
	img := image.NewPaletted(image.Rect(0, 0, f.W, f.H), nil)
	for i := 0; i < f.W*f.H; i++ {
		r, g, b := f.Pix[i*4], f.Pix[i*4+1], f.Pix[i*4+2]
		key := uint32(r)<<16 | uint32(g)<<8 | uint32(b)
		idx, ok := index[key]
		if !ok {
			if len(pal) < 256 {
				idx = uint8(len(pal))
				pal = append(pal, color.RGBA{r, g, b, 255})
				index[key] = idx
			} else {
				idx = uint8(pal.Index(color.RGBA{r, g, b, 255}))
				index[key] = idx
			}
		}
		img.Pix[i] = idx
	}
	if len(pal) == 0 {
		pal = append(pal, color.RGBA{0, 0, 0, 255})
	}
	img.Palette = pal
	return img, pal
}
