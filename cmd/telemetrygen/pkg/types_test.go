// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDurationWithInf(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    DurationWithInf
		wantErr bool
	}{
		{
			name:    "inf value",
			input:   "inf",
			want:    DurationWithInf(-1),
			wantErr: false,
		},
		{
			name:    "inf value case insensitive",
			input:   "INF",
			want:    DurationWithInf(-1),
			wantErr: false,
		},
		{
			name:    "regular duration",
			input:   "5s",
			want:    DurationWithInf(5 * time.Second),
			wantErr: false,
		},
		{
			name:    "complex duration",
			input:   "1h30m15s",
			want:    DurationWithInf(1*time.Hour + 30*time.Minute + 15*time.Second),
			wantErr: false,
		},
		{
			name:    "invalid duration",
			input:   "invalid",
			want:    DurationWithInf(0),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewDurationWithInf(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMustDurationWithInf(t *testing.T) {
	t.Run("valid inf", func(t *testing.T) {
		got := MustDurationWithInf("inf")
		assert.Equal(t, DurationWithInf(-1), got)
		assert.True(t, got.IsInf())
	})

	t.Run("valid duration", func(t *testing.T) {
		got := MustDurationWithInf("10s")
		assert.Equal(t, DurationWithInf(10*time.Second), got)
		assert.False(t, got.IsInf())
	})

	t.Run("panics on invalid", func(t *testing.T) {
		assert.Panics(t, func() {
			MustDurationWithInf("invalid")
		})
	})
}

func TestDurationWithInf_String(t *testing.T) {
	tests := []struct {
		name string
		d    DurationWithInf
		want string
	}{
		{
			name: "inf value",
			d:    DurationWithInf(-1),
			want: "inf",
		},
		{
			name: "regular duration",
			d:    DurationWithInf(5 * time.Second),
			want: "5s",
		},
		{
			name: "zero duration",
			d:    DurationWithInf(0),
			want: "0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.d.String())
		})
	}
}

func TestDurationWithInf_IsInf(t *testing.T) {
	tests := []struct {
		name string
		d    DurationWithInf
		want bool
	}{
		{
			name: "inf value",
			d:    DurationWithInf(-1),
			want: true,
		},
		{
			name: "regular duration",
			d:    DurationWithInf(5 * time.Second),
			want: false,
		},
		{
			name: "zero duration",
			d:    DurationWithInf(0),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.d.IsInf())
		})
	}
}

func TestDurationWithInf_Duration(t *testing.T) {
	tests := []struct {
		name string
		d    DurationWithInf
		want time.Duration
	}{
		{
			name: "inf value returns 0",
			d:    DurationWithInf(-1),
			want: 0,
		},
		{
			name: "regular duration",
			d:    DurationWithInf(5 * time.Second),
			want: 5 * time.Second,
		},
		{
			name: "zero duration",
			d:    DurationWithInf(0),
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.d.Duration())
		})
	}
}
