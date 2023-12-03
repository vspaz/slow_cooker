package cmd

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHashSampling(t *testing.T) {
	// With a samplingRate of 0.0 we never check.
	assertIterationsChecked(t, 100000, 0.0, 0)
	// With a samplingRate of 0.01 we check 1% of the values
	assertIterationsChecked(t, 100000, 0.01, 1000)
	// With a samplingRate of 0.1 we check 10% of the values
	assertIterationsChecked(t, 100000, 0.1, 10000)
	// With a samplingRate of 0.2 we check 20% of the values
	assertIterationsChecked(t, 100000, 0.2, 20000)
	// With a samplingRate of 0.5 we check 50% of the values
	assertIterationsChecked(t, 100000, 0.5, 50000)
	// With a samplingRate of 0.9 we check 90% of the values
	assertIterationsChecked(t, 100000, 0.9, 90000)
	// With a samplingRate of 0.999 we check 99.9% of the values
	assertIterationsChecked(t, 100000, 0.999, 99900)
	// With a samplingRate of 1.0 we check 100% of the values
	assertIterationsChecked(t, 100000, 1.0, 100000)
	// Somebody giving us a high sampleRate will still get 100% of the values
	assertIterationsChecked(t, 100000, 1.1, 100000)
}

func assertIterationsChecked(t *testing.T, iterations uint64, sampleRate float64, expectedChecks uint64) {
	actualIterationsCount := shouldCheckHashTest(sampleRate, iterations)
	assertWithin0and100Range(t, actualIterationsCount, expectedChecks, 10.0)
}

func shouldCheckHashTest(samplingRate float64, iterations uint64) (checked uint64) {
	for i := uint64(0); i < iterations; i++ {
		if shouldCheckHash(samplingRate) {
			checked++
		}
	}
	return checked
}

func assertWithin0and100Range(t *testing.T, actualValue uint64, expectedValue uint64, deltaPercentage float64) {
	assert.LessOrEqual(t, 0.0, deltaPercentage, fmt.Sprintf("deltaPercentage '%f' cannot be 0.0 or negative", deltaPercentage))
	assert.LessOrEqual(t, deltaPercentage, 100.0, fmt.Sprintf("deltaPercentage '%f' cannot be greater than 100.0.", deltaPercentage))

	delta := uint64(float64(expectedValue) * (0.01 * deltaPercentage))
	top := delta + expectedValue
	bottom := expectedValue - delta

	assert.True(
		t,
		actualValue >= bottom && actualValue <= top,
		fmt.Sprintf("%d is within %f of %d", actualValue, deltaPercentage, expectedValue))
}
