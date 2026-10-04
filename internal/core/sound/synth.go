package sound

import (
	"bytes"
	"encoding/binary"
	"math"
)

const rate = 44_100

// The start burst is made from the recorded keys, typed at a person's
// rhythm, so it sounds like the keyboard the clicks come from.

// samples are a shipped 16-bit mono WAV's samples, -1..1; nil when it is
// missing or not that format.
func samples(name string) []float64 {
	b, err := wavs.ReadFile("lazy-" + name + ".wav")
	if err != nil || len(b) < 44 || string(b[:4]) != "RIFF" {
		return nil
	}
	// Walk the chunks to "data": a recorder may write others before it.
	for i := 12; i+8 <= len(b); {
		id, size := string(b[i:i+4]), int(binary.LittleEndian.Uint32(b[i+4:i+8]))
		if id == "data" {
			end := min(len(b), i+8+size)
			out := make([]float64, 0, (end-i-8)/2)
			for j := i + 8; j+1 < end; j += 2 {
				out = append(out, float64(int16(binary.LittleEndian.Uint16(b[j:])))/math.MaxInt16)
			}
			return out
		}
		i += 8 + size + size%2
	}
	return nil
}

// typing is four recorded keys of a mechanical keyboard (lazy-start-1..4,
// the last the space bar) at a typing rhythm.
func typing() []float64 {
	buf := make([]float64, rate*45/100)
	for _, k := range []struct {
		name string
		ms   int
		gain float64
	}{{"start-1", 0, 0.85}, {"start-2", 95, 1}, {"start-3", 175, 0.8}, {"start-4", 270, 0.9}} {
		at := rate * k.ms / 1000
		for i, v := range samples(k.name) {
			if at+i < len(buf) {
				buf[at+i] += k.gain * v
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
