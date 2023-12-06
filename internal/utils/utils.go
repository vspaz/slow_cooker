package utils

import (
	"math/rand"
	"time"
)

// CalcTimeToWait calculates how many Nanoseconds to wait between actions.
func CalcTimeToWait(qps *int) time.Duration {
	return time.Duration(int(time.Second) / *qps)
}

// Sample Rate is between [0.0, 1.0] and determines what percentage of request bodies
// should be checked that their hash matches a known hash.
func ShouldCheckHash(sampleRate float64) bool {
	return rand.Float64() < sampleRate
}
