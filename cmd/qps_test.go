package cmd

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestQpsCalcOk(t *testing.T) {
	// At 100 qps, we expect to wait 10 milliseconds
	assertWaitMs(t, 10, 100)
	// At 1000 qps, we expect to wait 1 millisecond
	assertWaitMs(t, 1, 1000)
	// At 150 qps, we expect to wait 6.666 milliseconds
	assertWaitMs(t, 6.666666, 150)
	// At 134 qps, we expect to wait 7.462 milliseconds
	assertWaitMs(t, 7.462686, 134)
}

func assertWaitMs(t *testing.T, expectedWaitTimeMs float64, targetQPS int) {
	expected := time.Duration(expectedWaitTimeMs * float64(time.Millisecond))
	got := CalcTimeToWait(&targetQPS)
	assert.Equal(
		t,
		expected,
		got,
		fmt.Sprintf("For %d qps, expected to wait %s, instead we wait %s", targetQPS, expected, got))
}
