// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestBatchLogs_ConsumeLogs(t *testing.T) {
	type debugMsg struct {
		text         string
		droppedCount int64
	}
	profilingDropped := debugMsg{text: "Profiling data is not allowed", droppedCount: 4}
	logsDropped := debugMsg{text: "Log data is not allowed", droppedCount: 5}
	tests := []struct {
		name             string
		profilingEnabled bool
		logsEnabled      bool
		in               string
		out              []string
		wantDropped      []debugMsg
	}{
		{
			name:             "profiling_only_both_enabled",
			profilingEnabled: true,
			logsEnabled:      true,
			in:               "profiling_only.yaml",
			out:              []string{"profiling_only.yaml"},
		},
		{
			name:             "profiling_only_profiling_enabled",
			profilingEnabled: true,
			in:               "profiling_only.yaml",
			out:              []string{"profiling_only.yaml"},
		},
		{
			name:        "profiling_only_profiling_disabled",
			logsEnabled: true,
			in:          "profiling_only.yaml",
			out:         []string{},
			wantDropped: []debugMsg{profilingDropped},
		},
		{
			name:             "regular_logs_only_both_enabled",
			profilingEnabled: true,
			logsEnabled:      true,
			in:               "regular_logs_only.yaml",
			out:              []string{"regular_logs_only.yaml"},
		},
		{
			name:        "regular_logs_only_logs_enabled",
			logsEnabled: true,
			in:          "regular_logs_only.yaml",
			out:         []string{"regular_logs_only.yaml"},
		},
		{
			name:             "regular_logs_only_logs_disabled",
			profilingEnabled: true,
			in:               "regular_logs_only.yaml",
			out:              []string{},
			wantDropped:      []debugMsg{logsDropped},
		},
		{
			name:             "combined_both_enabled",
			profilingEnabled: true,
			logsEnabled:      true,
			in:               "combined.yaml",
			out:              []string{"regular_logs_only.yaml", "profiling_only.yaml"},
		},
		{
			name:        "combined_profiling_disabled",
			logsEnabled: true,
			in:          "combined.yaml",
			out:         []string{"regular_logs_only.yaml"},
			wantDropped: []debugMsg{profilingDropped},
		},
		{
			name:             "combined_logs_disabled",
			profilingEnabled: true,
			in:               "combined.yaml",
			out:              []string{"profiling_only.yaml"},
			wantDropped:      []debugMsg{logsDropped},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := &consumertest.LogsSink{}
			core, obs := observer.New(zapcore.DebugLevel)
			logger := zap.New(core)

			consumer := &perScopeBatcher{
				profilingEnabled: tt.profilingEnabled,
				logsEnabled:      tt.logsEnabled,
				logger:           logger,
				next:             sink,
			}

			logs, err := golden.ReadLogs("testdata/batchperscope/" + tt.in)
			require.NoError(t, err)

			err = consumer.ConsumeLogs(context.Background(), logs)
			assert.NoError(t, err)

			require.Equal(t, len(tt.out), len(sink.AllLogs()))
			for i, out := range tt.out {
				expected, err := golden.ReadLogs("testdata/batchperscope/" + out)
				require.NoError(t, err)
				assert.NoError(t, plogtest.CompareLogs(expected, sink.AllLogs()[i]))
			}

			require.Equal(t, len(tt.wantDropped), obs.Len())
			for _, entry := range tt.wantDropped {
				filtered := obs.FilterMessage(entry.text)
				require.Equal(t, 1, filtered.Len())
				assert.Equal(t, entry.droppedCount, filtered.All()[0].ContextMap()["dropped_records"])
			}
		})
	}
}
