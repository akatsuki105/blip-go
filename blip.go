package blip

import (
	"errors"
	"unsafe"
)

type fixed_t = uint64

type buf_t = int

// Sample buffer that resamples to output rate and accumulates samples until they're read out
type Blip struct {
	factor     fixed_t
	offset     fixed_t
	avail      int
	size       int
	integrator int
	buffer     []buf_t
}

func clamp(n int) int {
	if n&0xffff != n {
		n = (n >> 16) ^ 32767
	}
	return n
}

func New(size uint) *Blip {
	m := &Blip{
		factor: timeunit / blipMaxRatio,
		size:   int(size),
		buffer: make([]buf_t, size+bufExtra),
	}
	m.Clear()

	return m
}

func (b *Blip) Delete() {
	if b != nil {
		b = nil
	}
}

func (b *Blip) SetRates(clockRate, sampleRate float64) error {
	factor := timeunit * sampleRate / clockRate
	b.factor = fixed_t(factor)

	/* Fails if clock_rate exceeds maximum, relative to sample_rate */
	if !(0 <= factor-float64(b.factor) && factor-float64(b.factor) < 1) {
		return errors.New("clockRate exceeds maximum, relative to sampleRate")
	}

	/* Avoid requiring math.h. Equivalent to
	m->factor = (int) ceil( factor ) */
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

func (b *Blip) ClocksNeeded(samples uint) int {
	if b.avail+int(samples) > b.size {
		return 0
	}

	needed := fixed_t(samples) * timeunit
	if needed < b.offset {
		return 0
	}

	return int((needed - b.offset + b.factor - 1) / b.factor)
}

func (b *Blip) EndFrame(t uint) error {
	off := fixed_t(t)*b.factor + b.offset
	b.avail += int(off >> timebits)
	b.offset = off & (timeunit - 1)

	if b.avail > b.size {
		return errors.New("buffer size was exceeded")
	}
	return nil
}

func (b *Blip) SamplesAvail() int {
	return b.avail
}

func (b *Blip) removeSamples(count int) {
	remain := b.avail + bufExtra - count
	b.avail -= count

	for i := 0; i < remain; i++ {
		b.buffer[i] = b.buffer[count+i]
	}
	for i := 0; i < count; i++ {
		b.buffer[remain+i] = 0
	}
}

func (b *Blip) ReadSamples(out unsafe.Pointer, count int, stereo bool) int {
	if count < 0 {
		return 0
	}

	if count > b.avail {
		count = b.avail
	}

	if count > 0 {
		step := 1
		if stereo {
			step = 2
		}
		sum := b.integrator

		for i := 0; i < count; i++ {
			sample := sum >> deltaBits

			sum += b.buffer[i]

			sample = clamp(sample)

			*(*int)(out) = sample
			out = unsafe.Add(out, step)

			/* High-pass filter */
			sum -= sample << (deltaBits - bassShift)
		}

		b.integrator = sum

		b.removeSamples(count)
	}

	return count
}

func (b *Blip) AddDelta(time uint, delta int) error {
	fixed := (time*uint(b.factor) + uint(b.offset)) >> preshift
	out := b.buffer[b.avail+int(fixed>>fracBits):]

	phaseShift := fracBits - phaseBits
	phase := fixed >> phaseShift & (phaseCount - 1)
	in := blStep[phase]

	interp := int(fixed >> (phaseShift - deltaBits) & (deltaUnit - 1))
	delta2 := (delta * interp) >> deltaBits
	delta -= delta2

	if b.avail+int(fixed>>fracBits) > b.size+2 {
		return errors.New("buffer size was exceeded")
	}

	inNext := blStep[phase+1]
	for i := 0; i < 8; i++ {
		out[i] += int(in[i])*delta + int(inNext[i])*delta2
	}

	in = blStep[phaseCount-phase]
	inPrev := blStep[phaseCount-phase-1]
	for i := 0; i < 8; i++ {
		out[8+i] += int(in[7-i])*delta + int(inPrev[7-i])*delta2
	}

	return nil
}
