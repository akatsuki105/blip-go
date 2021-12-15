package blip

import (
	"errors"
	"unsafe"
)

type buf_t = int32

// Sample buffer that resamples to output rate and accumulates samples until they're read out
type Blip struct {
	factor     uint64
	offset     uint64
	avail      int32
	size       int32
	integrator int32
	buffer     []buf_t
}

// Creates new buffer that can hold at most sample_count samples. Sets rates
// so that there are blip_max_ratio clocks per sample. Returns pointer to new
// buffer, or NULL if insufficient memory.
func New(size uint) *Blip {
	m := &Blip{
		factor: timeUnit / MaxRatio,
		size:   int32(size),
		buffer: make([]buf_t, size+bufExtra),
	}
	m.Clear()

	return m
}

// Frees buffer. No effect if NULL is passed.
func (b *Blip) Delete() {
	if b != nil {
		b = nil
	}
}

// Sets approximate input clock rate and output sample rate. For every
// clock_rate input clocks, approximately sample_rate samples are generated.
func (b *Blip) SetRates(clockRate, sampleRate float64) error {
	factor := timeUnit * sampleRate / clockRate
	b.factor = uint64(factor)

	if !(0 <= factor-float64(b.factor) && factor-float64(b.factor) < 1) {
		return errors.New("clockRate exceeds maximum, relative to sampleRate")
	}

	/* Avoid requiring math.h. Equivalent to m->factor = (int) ceil( factor ) */
	if float64(b.factor) < factor {
		b.factor++
	}

	/* At this point, factor is most likely rounded up, but could still
	have been rounded down in the floating-point calculation. */

	return nil
}

func (b *Blip) Clear() {
	/* We could set offset to 0, factor/2, or factor-1. 0 is suitable if
	factor is rounded up. factor-1 is suitable if factor is rounded down.
	Since we don't know rounding direction, factor/2 accommodates either,
	with the slight loss of showing an error in half the time. Since for
	a 64-bit factor this is years, the halving isn't a problem. */

	b.offset = b.factor / 2
	b.avail = 0
	b.integrator = 0
	for i := range b.buffer {
		b.buffer[i] = 0
	}
}

// Length of time frame, in clocks, needed to make sample_count additional samples available.
func (b *Blip) ClocksNeeded(samples uint) int {
	if b.avail+int32(samples) > b.size {
		return 0
	}

	needed := uint64(samples) * timeUnit
	if needed < b.offset {
		return 0
	}

	return int((needed - b.offset + b.factor - 1) / b.factor)
}

// Makes input clocks before clock_duration available for reading as output
// samples. Also begins new time frame at clock_duration, so that clock time 0 in
// the new time frame specifies the same clock as clock_duration in the old time
// frame specified. Deltas can have been added slightly past clock_duration (up to
// however many clocks there are in two output samples).
func (b *Blip) EndFrame(t uint) error {
	off := uint64(t)*b.factor + b.offset
	b.avail += int32(off >> timeBits)
	b.offset = off & (timeUnit - 1)

	if b.avail > b.size {
		return errors.New("buffer size was exceeded")
	}
	return nil
}

// Number of buffered samples available for reading.
func (b *Blip) SamplesAvail() int {
	return int(b.avail)
}

func (b *Blip) removeSamples(count int) {
	remain := b.avail + int32(bufExtra) - int32(count)
	b.avail -= int32(count)

	for i := 0; i < int(remain); i++ {
		b.buffer[i] = b.buffer[count+i]
	}
	for i := 0; i < count; i++ {
		b.buffer[remain+int32(i)] = 0
	}
}

func (b *Blip) ReadSamples(out unsafe.Pointer, count int, stereo bool) int {
	if count < 0 {
		return 0
	}

	if int32(count) > b.avail {
		count = int(b.avail)
	}

	if count > 0 {
		step := 1
		if stereo {
			step = 2
		}
		sum := b.integrator

		for i := 0; i < count; i++ {
			s := sum >> deltaBits // Eliminate fraction

			sum += b.buffer[i]

			s = clamp(s)

			*(*int16)(out) = int16(s)
			out = unsafe.Add(out, step*2)

			// High-pass filter
			sum -= s << (deltaBits - bassShift)
		}

		b.integrator = sum

		b.removeSamples(count)
	}

	return count
}

func (b *Blip) AddDelta(time uint, delta int) error {
	fixed := uint32((uint64(time)*b.factor + b.offset) >> preShift)
	out := b.buffer[b.avail+int32(fixed>>fracBits):]

	phaseShift := fracBits - phaseBits
	phase := (fixed >> phaseShift) & (phaseCount - 1)
	in := blStep[phase]

	interp := int((fixed >> (phaseShift - deltaBits)) & (deltaUnit - 1))
	delta2 := (delta * interp) >> deltaBits
	delta -= delta2

	if b.avail+int32(fixed>>fracBits) > b.size+endFrameExtra {
		return errors.New("buffer size was exceeded")
	}

	next := blStep[phase+1]
	for i := 0; i < 8; i++ {
		out[i] += int32(int(in[i])*delta + int(next[i])*delta2)
	}

	in = blStep[phaseCount-phase]
	prev := blStep[phaseCount-phase-1]
	for i := 0; i < 8; i++ {
		out[8+i] += int32(int(in[7-i])*delta + int(prev[7-i])*delta2)
	}

	return nil
}

// Same as blip_add_delta(), but uses faster, lower-quality synthesis.
func (b *Blip) AddDeltaFast(time uint, delta int) error {
	fixed := uint((uint64(time)*b.factor + b.offset) >> preShift)
	out := b.buffer[b.avail+int32(fixed>>fracBits):]

	interp := int((fixed >> (fracBits - deltaBits)) & (deltaUnit - 1))
	delta2 := delta * interp

	if b.avail+int32(fixed>>fracBits) > b.size+endFrameExtra {
		return errors.New("buffer size was exceeded")
	}

	out[7] += int32(delta*deltaUnit - delta2)
	out[8] += int32(delta2)
	return nil
}
