package io

// Speaker turns the PIT channel 2 / port 61h state into samples: an ideal
// square wave at the programmed frequency when the timer drives the
// speaker, a DC level when the data bit is driven directly, filtered by a
// one-pole low-pass so gate changes do not click.
type Speaker struct {
	pit   *PIT
	rate  float64
	phase uint32
	lp    int32
}

// speakerAmp is the peak level of the speaker in 16-bit sample units.
const speakerAmp = 6000

// NewSpeaker returns a speaker fed by pit at the given sample rate.
func NewSpeaker(pit *PIT, rate int) *Speaker {
	return &Speaker{pit: pit, rate: float64(rate)}
}

// Render adds len(dst) samples to dst.
func (s *Speaker) Render(dst []int32) {
	gate, data, reload, square := s.pit.SpeakerState()
	inc := uint32(float64(PITClock) / float64(reload) / s.rate * 4294967296.0)
	for i := range dst {
		var v int32
		switch {
		case gate && data && square:
			if s.phase < 1<<31 {
				v = speakerAmp
			} else {
				v = -speakerAmp
			}
			s.phase += inc
		case data:
			v = speakerAmp
		}
		s.lp += (v - s.lp) >> 3
		dst[i] += s.lp
	}
}
