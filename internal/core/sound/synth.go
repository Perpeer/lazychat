package sound

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
)

// The keyboard sounds are made here rather than recorded: an old
// buckling-spring key is two knocks — the spring snapping, bright and
// short, then the keycap bottoming out, lower and duller — with the
// spring's faint ring over them.

const rate = 44_100

// knock is one transient at the start of buf: noise and a tone at freq,
// both dying away over decay seconds, gain loud.
func knock(buf []float64, at int, freq, decay, gain float64, rng *rand.Rand) {
	for i := at; i < len(buf); i++ {
		t := float64(i-at) / rate
		env := math.Exp(-t / decay)
		if env < 1e-4 {
			break
		}
		noise := rng.Float64()*2 - 1
		buf[i] += gain * env * (0.55*noise + 0.45*math.Sin(2*math.Pi*freq*t))
	}
}

// keyClick is one key press; variant shifts its pitch and timing a little
// so fast typing does not sound like one sample repeated.
func keyClick(variant int) []float64 {
	rng := rand.New(rand.NewPCG(uint64(variant)+1, 7))
	shift := 1 + 0.06*float64(variant-1)
	buf := make([]float64, rate*55/1000)
	knock(buf, 0, 3800*shift, 0.0025, 0.9, rng)                    // the spring snaps
	knock(buf, rate*(16+variant)/1000, 750*shift, 0.006, 0.7, rng) // the cap bottoms out
	for i := range buf {                                           // the spring's ring
		t := float64(i) / rate
		buf[i] += 0.08 * math.Exp(-t/0.02) * math.Sin(2*math.Pi*2600*shift*t)
	}
	return buf
}

// typing is a short burst of keys, Lazy starting on a prompt.
func typing() []float64 {
	buf := make([]float64, rate*38/100)
	for n, ms := range []int{0, 85, 160, 255} {
		click := keyClick(n%3 + 1)
		at := rate * ms / 1000
		gain := []float64{0.8, 1, 0.75, 0.95}[n]
		for i, v := range click {
			if at+i < len(buf) {
				buf[at+i] += gain * v
			}
		}
	}
	return buf
}

// wav is samples as a 16-bit mono WAV file, at volume.
func wav(samples []float64, volume float64) []byte {
	var data bytes.Buffer
	for _, v := range samples {
		v = math.Max(-1, math.Min(1, v*volume))
		_ = binary.Write(&data, binary.LittleEndian, int16(v*math.MaxInt16))
	}
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+data.Len()))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(rate), uint32(rate * 2), uint16(2), uint16(16)} {
		_ = binary.Write(&b, binary.LittleEndian, v) // PCM, mono, 16-bit
	}
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(data.Len()))
	b.Write(data.Bytes())
	return b.Bytes()
}
