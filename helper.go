package blip

func clamp(n int32) int32 {
	if int(int16(n)) != int(n) {
		n = (n >> 16) ^ maxSample
	}
	return n
}
