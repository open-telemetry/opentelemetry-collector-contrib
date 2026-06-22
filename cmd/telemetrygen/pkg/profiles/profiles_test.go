// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/config"
	types "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *Config
		wantErrMessage string
	}{
		{
			name: "No duration or profiles count",
			cfg: &Config{
				Config: config.Config{
					WorkerCount: 1,
				},
				TraceID: "123",
			},
			wantErrMessage: "either `profiles` or `duration` must be greater than 0",
		},
		{
			name: "TraceID invalid",
			cfg: &Config{
				Config: config.Config{
					WorkerCount: 1,
				},
				NumProfiles:     5,
				SampleCount:     1,
				StackDepth:      1,
				UniqueFunctions: 1,
				ProfileDuration: 1 * time.Second,
				TraceID:         "123",
			},
			wantErrMessage: "TraceID must be a 32 character hex string, like: 'ae87dadd90e9935a4bc9660628efd569'",
		},
		{
			name: "SpanID invalid",
			cfg: &Config{
				Config: config.Config{
					WorkerCount: 1,
				},
				NumProfiles:     5,
				SampleCount:     1,
				StackDepth:      1,
				UniqueFunctions: 1,
				ProfileDuration: 1 * time.Second,
				TraceID:         "ae87dadd90e9935a4bc9660628efd569",
				SpanID:          "123",
			},
			wantErrMessage: "SpanID must be a 16 character hex string, like: '5828fa4960140870'",
		},
		{
			name: "LoadSize negative",
			cfg: &Config{
				Config: config.Config{
					WorkerCount: 1,
					LoadSize:    -1,
				},
				NumProfiles:     5,
				SampleCount:     1,
				StackDepth:      1,
				UniqueFunctions: 1,
				ProfileDuration: 1 * time.Second,
			},
			wantErrMessage: "load size must be non-negative, found -1",
		},
		{
			name: "SampleCount invalid",
			cfg: &Config{
				Config: config.Config{
					WorkerCount: 1,
				},
				NumProfiles:     5,
				SampleCount:     0,
				StackDepth:      1,
				UniqueFunctions: 1,
			},
			wantErrMessage: "sample-count must be greater than 0",
		},
		{
			name: "StackDepth invalid",
			cfg: &Config{
				Config: config.Config{
					WorkerCount: 1,
				},
				NumProfiles:     5,
				SampleCount:     1,
				StackDepth:      0,
				UniqueFunctions: 1,
			},
			wantErrMessage: "stack-depth must be greater than 0",
		},
		{
			name: "UniqueFunctions invalid",
			cfg: &Config{
				Config: config.Config{
					WorkerCount: 1,
				},
				NumProfiles:     5,
				SampleCount:     1,
				StackDepth:      1,
				UniqueFunctions: 0,
			},
			wantErrMessage: "unique-functions must be greater than 0",
		},
		{
			name: "ProfileDuration negative",
			cfg: &Config{
				Config: config.Config{
					WorkerCount: 1,
				},
				NumProfiles:     5,
				SampleCount:     1,
				StackDepth:      1,
				UniqueFunctions: 1,
				ProfileDuration: -1 * time.Second,
			},
			wantErrMessage: "profile-duration must be non-negative",
		},
	}

	mockExp := func() (profileExporter, error) {
		return &mockProfileExporter{}, nil
	}
	logger := zap.NewNop()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(tt.cfg, mockExp, logger)
			require.EqualError(t, err, tt.wantErrMessage)
		})
	}
}

func TestDefaultConfiguration(t *testing.T) {
	cfg := NewConfig()

	assert.Equal(t, types.DurationWithInf(0), cfg.TotalDuration, "Default TotalDuration should be 0")
	assert.Equal(t, 0, cfg.NumProfiles, "Default NumProfiles should be 0")
	assert.Equal(t, float64(1), cfg.Rate, "Default Rate should be 1")
	assert.Equal(t, 10, cfg.SampleCount, "Default SampleCount should be 10")
	assert.Equal(t, 5, cfg.StackDepth, "Default StackDepth should be 5")
	assert.Equal(t, 20, cfg.UniqueFunctions, "Default UniqueFunctions should be 20")
	assert.Equal(t, 10*time.Second, cfg.ProfileDuration, "Default ProfileDuration should be 10s")
	assert.Empty(t, cfg.TraceID, "Default TraceID should be empty")
	assert.Empty(t, cfg.SpanID, "Default SpanID should be empty")
}
