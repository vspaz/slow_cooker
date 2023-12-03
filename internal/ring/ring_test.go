package ring

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRingOk(t *testing.T) {
	r := New(5)
	assert.Equal(t, len(r.Items), 5)

	for i := 1; i <= 10; i++ {
		r.Push(i)
	}

	assert.Equal(t, []int{6, 7, 8, 9, 10}, r.Items)

	// Make a ring of 6 items
	r = New(6)
	// Push 7 items
	r.Push(1)
	r.Push(10)
	r.Push(99)
	r.Push(50)
	r.Push(77)
	r.Push(83)
	r.Push(2)
	// The oldest item should be gone
	assert.Equal(t, []int{2, 10, 99, 50, 77, 83}, r.Items)
}
