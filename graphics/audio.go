package graphics

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"

	emio "assembly-emulator/emulator/io"
	"assembly-emulator/machine"
)

// startAudio connects the machine's mixer to an Ebiten audio player and
// returns a function that stops it. Unthrottled runs produce audio faster
// than real time, so they stay silent.
func startAudio(m *machine.Machine) func() {
	if m.Opts.Unlimited {
		return func() {}
	}
	ctx := audio.CurrentContext()
	if ctx == nil {
		ctx = audio.NewContext(emio.SampleRate)
	}
	m.Sound.Enable(emio.SampleRate / 5) // 200 ms of buffering
	p, err := ctx.NewPlayer(m.Sound)
	if err != nil {
		return func() {}
	}
	p.SetBufferSize(50 * time.Millisecond)
	p.Play()
	return func() { p.Close() }
}
