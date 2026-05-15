// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
)

func TestOTLPDataSenderConstructors(t *testing.T) {
	tests := []struct {
		name        string
		opts        []OTLPDataSenderOption
		wantTimeout time.Duration
	}{
		{
			name:        "no options leaves timeout at zero",
			opts:        nil,
			wantTimeout: 0,
		},
		{
			name:        "WithTimeout sets timeout",
			opts:        []OTLPDataSenderOption{WithTimeout(30 * time.Second)},
			wantTimeout: 30 * time.Second,
		},
		{
			name:        "WithTimeout(0) is a no-op",
			opts:        []OTLPDataSenderOption{WithTimeout(0)},
			wantTimeout: 0,
		},
		{
			name:        "last WithTimeout wins",
			opts:        []OTLPDataSenderOption{WithTimeout(10 * time.Second), WithTimeout(20 * time.Second)},
			wantTimeout: 20 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("traces", func(t *testing.T) {
				s, ok := NewOTLPTraceDataSender("localhost", 4317, tt.opts...).(*otlpTraceDataSender)
				require.True(t, ok)
				assert.Equal(t, tt.wantTimeout, s.timeout)
			})
			t.Run("metrics", func(t *testing.T) {
				s, ok := NewOTLPMetricDataSender("localhost", 4317, tt.opts...).(*otlpMetricsDataSender)
				require.True(t, ok)
				assert.Equal(t, tt.wantTimeout, s.timeout)
			})
			t.Run("logs", func(t *testing.T) {
				s, ok := NewOTLPLogsDataSender("localhost", 4317, tt.opts...).(*otlpLogsDataSender)
				require.True(t, ok)
				assert.Equal(t, tt.wantTimeout, s.timeout)
			})
		})
	}
}

func TestOTLPDataSenderFillConfigTimeout(t *testing.T) {
	factory := otlpexporter.NewFactory()
	defaultTimeout := factory.CreateDefaultConfig().(*otlpexporter.Config).TimeoutConfig.Timeout

	tests := []struct {
		name        string
		timeout     time.Duration
		wantTimeout time.Duration
	}{
		{
			name:        "zero timeout preserves exporter default",
			timeout:     0,
			wantTimeout: defaultTimeout,
		},
		{
			name:        "non-zero timeout overrides exporter default",
			timeout:     30 * time.Second,
			wantTimeout: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &otlpDataSender{
				DataSenderBase: DataSenderBase{Host: "localhost", Port: 4317},
				timeout:        tt.timeout,
			}
			cfg := s.fillConfig(factory.CreateDefaultConfig().(*otlpexporter.Config))
			assert.Equal(t, tt.wantTimeout, cfg.TimeoutConfig.Timeout)
		})
	}
}
