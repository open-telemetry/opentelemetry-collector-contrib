// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshalText(t *testing.T) {
	tests := []struct {
		str  []string
		unit Unit
		err  bool
	}{
		{
			str:  []string{"ms", "Ms", "MS"},
			unit: Milliseconds,
		},
		{
			str:  []string{"s", "S"},
			unit: Seconds,
		},
		{
			str: []string{"h", "H"},
			err: true,
		},
		{
			str: []string{""},
			err: true,
		},
	}

	for _, tt := range tests {
		for _, str := range tt.str {
			t.Run(str, func(t *testing.T) {
				var u Unit
				err := u.UnmarshalText([]byte(str))
				if tt.err {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.unit, u)
				}
			})
		}
	}
}

func TestUnmarshalTextNilUnit(t *testing.T) {
	lvl := (*Unit)(nil)
	assert.Error(t, lvl.UnmarshalText([]byte(MillisecondsStr)))
}

func TestUnitStringMarshal(t *testing.T) {
	tests := []struct {
		str  string
		unit Unit
		err  bool
	}{
		{
			str:  SecondsStr,
			unit: Seconds,
		},
		{
			str:  MillisecondsStr,
			unit: Milliseconds,
		},
	}
	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			assert.Equal(t, tt.str, tt.unit.String())
			got, err := tt.unit.MarshalText()
			assert.NoError(t, err)
			assert.Equal(t, tt.str, string(got))
		})
	}
}
