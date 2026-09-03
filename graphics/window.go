// Package graphics is the Ebiten front end: it shows the emulated screen
// (text or graphics mode), forwards keyboard events as scancodes, and can
// record GIFs of the display.
package graphics

import (
	"fmt"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	emio "assembly-emulator/emulator/io"
	"assembly-emulator/machine"
)

// Scale is the initial window scale factor for 320x200.
const Scale = 3

type game struct {
	m        *machine.Machine
	frame    emio.Frame
	lastN    uint64
	img      *ebiten.Image
	done     chan error
	err      error
	mu       sync.Mutex
	finished bool
	title    string
}

// Run opens a window and runs the machine until the program exits, the
// window is closed, or F12 is pressed.
func Run(m *machine.Machine, title string) error {
	m.RenderFrames = true
	g := &game{m: m, done: make(chan error, 1), title: title}
	ebiten.SetWindowTitle(title)
	ebiten.SetWindowSize(320*Scale, 200*Scale)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetScreenClearedEveryFrame(false)
	stopAudio := startAudio(m)
	defer stopAudio()
	go func() {
		err := m.Run()
		g.mu.Lock()
		g.finished = true
		g.err = err
		g.mu.Unlock()
	}()
	err := ebiten.RunGame(g)
	m.Stop()
	if err != nil && err != ebiten.Termination {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.err
}

func (g *game) Update() error {
	g.mu.Lock()
	finished := g.finished
	g.mu.Unlock()
	if finished && g.m.QuitRequested() {
		return ebiten.Termination
	}
	if finished && g.m.Exited() {
		// Keep the final frame visible until the window is closed or a
		// key is pressed.
		if len(inpututil.AppendJustPressedKeys(nil)) > 0 {
			return ebiten.Termination
		}
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF12) {
		return ebiten.Termination
	}
	for _, k := range inpututil.AppendJustPressedKeys(nil) {
		if sc, ok := keymap[k]; ok {
			g.m.KeyDown(sc.code, sc.ext)
		}
	}
	for _, k := range inpututil.AppendJustReleasedKeys(nil) {
		if sc, ok := keymap[k]; ok {
			g.m.KeyUp(sc.code, sc.ext)
		}
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	n := g.m.Frame(&g.frame)
	if g.frame.W == 0 {
		return
	}
	if g.img == nil || g.img.Bounds().Dx() != g.frame.W || g.img.Bounds().Dy() != g.frame.H {
		g.img = ebiten.NewImage(g.frame.W, g.frame.H)
		g.lastN = 0
	}
	if n != g.lastN {
		g.img.WritePixels(g.frame.Pix)
		g.lastN = n
	}
	// Letterbox to the window keeping the 4:3 aspect ratio.
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	scale := float64(sw) / 4.0
	if float64(sh)/3.0 < scale {
		scale = float64(sh) / 3.0
	}
	dw, dh := 4*scale, 3*scale
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(dw/float64(g.frame.W), dh/float64(g.frame.H))
	op.GeoM.Translate((float64(sw)-dw)/2, (float64(sh)-dh)/2)
	op.Filter = ebiten.FilterNearest
	screen.Fill(colorBlack)
	screen.DrawImage(g.img, op)
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

var colorBlack = ebitenBlack()

func (g *game) String() string { return fmt.Sprintf("window %q", g.title) }
