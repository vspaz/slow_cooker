package window

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMeanOk(t *testing.T) {
	assert.Equal(t, Mean([]int{}), 0)
	assert.Equal(t, Mean([]int{10, 20, 30, 40}), 25)
	assert.Equal(t, Mean([]int{8, 6, 5, 1000}), 254)
	assert.Equal(t, Mean([]int{0, 7, 10, 9, 1000000}), 200005)
}

func TestCalculateChangeIndicatorOk(t *testing.T) {
	data := []int{0, 7, 10, 9}
	assert.Equal(t, CalculateChangeIndicator(data, 1000000), "+++")
	assert.Equal(t, CalculateChangeIndicator(data, 1000), "++")
	assert.Equal(t, CalculateChangeIndicator(data, 100), "+")
	assert.Equal(t, CalculateChangeIndicator(data, 10), "")
	assert.Equal(t, CalculateChangeIndicator(data, 0), "-")

	data = []int{1000000, 1000000, 1000000, 1000000}
	assert.Equal(t, CalculateChangeIndicator(data, 1000000), "")
	assert.Equal(t, CalculateChangeIndicator(data, 100000), "-")
	assert.Equal(t, CalculateChangeIndicator(data, 10000), "--")
	assert.Equal(t, CalculateChangeIndicator(data, 1000), "---")

	data = []int{0, 0, 0, 0, 0}
	assert.Equal(t, CalculateChangeIndicator(data, 0), "")
}
