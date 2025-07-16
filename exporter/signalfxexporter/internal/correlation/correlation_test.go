// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package correlation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/metadata"
)

func TestTrackerAddSpans(t *testing.T) {
	tracker := NewTracker(
		DefaultConfig(),
		"abcd",
		exportertest.NewNopSettings(metadata.Type),
	)

	err := tracker.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, tracker.correlation, "correlation context should be set")

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	attr := rs.Resource().Attributes()
	attr.PutStr("host.name", "localhost")

	// Add empty first, should ignore.
	assert.NoError(t, tracker.ProcessTraces(context.Background(), ptrace.NewTraces()))
	assert.Nil(t, tracker.traceTracker)

	assert.NoError(t, tracker.ProcessTraces(context.Background(), traces))

	assert.NotNil(t, tracker.traceTracker, "trace tracker should be set")

	assert.NoError(t, tracker.Shutdown(context.Background()))
}

func TestTrackerStart(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "invalid http client settings fails",
			config: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "localhost:9090",
					TLS: configtls.ClientConfig{
						Config: configtls.Config{
							CAFile: "/non/existent",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "failed to create correlation API client: failed to load TLS config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewTracker(
				tt.config,
				"abcd",
				exportertest.NewNopSettings(metadata.Type),
			)

			err := tracker.Start(context.Background(), componenttest.NewNopHost())

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.ErrorContains(t, err, tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}

			assert.NoError(t, tracker.Shutdown(context.Background()))
		})
	}
}
