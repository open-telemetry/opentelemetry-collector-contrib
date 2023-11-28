package delta_test

import (
	"math/rand"
	"testing"
)

func TestSum(t *testing.T) {
	type Case struct {
	}

	t.Run("int", func(t *testing.T) {
		var samples [32]int64
		for i := range samples {
			samples[i] = int64(rand.Uint64())
		}
	})
}
