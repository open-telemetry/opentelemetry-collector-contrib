// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
)

func TestHiLo(t *testing.T) {
	type T struct {
		int int
		str string
	}

	a := T{int: 0, str: "foo"}
	b := T{int: 1, str: "bar"}

	{
		hi, lo := expo.HiLo(a, b, func(v T) int { return v.int })
		assert.Equal(t, a, lo)
		assert.Equal(t, b, hi)
	}

	{
		hi, lo := expo.HiLo(a, b, func(v T) string { return v.str })
		assert.Equal(t, b, lo)
		assert.Equal(t, a, hi)
	}

	{
		hi, lo := expo.HiLo(a, b, func(T) int { return 0 })
		assert.Equal(t, a, hi)
		assert.Equal(t, b, lo)
	}
}
