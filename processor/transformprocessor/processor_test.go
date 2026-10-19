// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

func TestProcessLogsWithoutFlatten(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.LogStatements = []common.ContextStatements{
		{
			Context: "log",
			Statements: []string{
				`set(resource.attributes["host.name"], attributes["host.name"])`,
				`delete_key(attributes, "host.name")`,
			},
		},
	}
	sink := new(consumertest.LogsSink)
	p, err := factory.CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), oCfg, sink)
	require.NoError(t, err)

	input, err := golden.ReadLogs(filepath.Join("testdata", "logs", "input.yaml"))
	require.NoError(t, err)
	expected, err := golden.ReadLogs(filepath.Join("testdata", "logs", "expected-without-flatten.yaml"))
	require.NoError(t, err)

	require.NoError(t, p.ConsumeLogs(t.Context(), input))

	actual := sink.AllLogs()
	require.Len(t, actual, 1)

	require.NoError(t, plogtest.CompareLogs(expected, actual[0]))
}

func TestProcessLogsWithFlatten(t *testing.T) {
	tests := []struct {
		name         string
		flattenLogs  bool
		groupFlatten bool
	}{
		{name: "flatten.logs", flattenLogs: true},
		{name: "flatten", groupFlatten: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)
			oCfg.Flatten.Logs = tt.flattenLogs
			oCfg.LogStatements = []common.ContextStatements{
				{
					Context: "log",
					Flatten: tt.groupFlatten,
					Statements: []string{
						`set(resource.attributes["host.name"], attributes["host.name"])`,
						`delete_key(attributes, "host.name")`,
					},
				},
			}
			sink := new(consumertest.LogsSink)
			p, err := factory.CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), oCfg, sink)
			require.NoError(t, err)

			input, err := golden.ReadLogs(filepath.Join("testdata", "logs", "input.yaml"))
			require.NoError(t, err)
			expected, err := golden.ReadLogs(filepath.Join("testdata", "logs", "expected-with-flatten.yaml"))
			require.NoError(t, err)

			require.NoError(t, p.ConsumeLogs(t.Context(), input))

			actual := sink.AllLogs()
			require.Len(t, actual, 1)

			require.NoError(t, plogtest.CompareLogs(expected, actual[0]))
		})
	}
}

func BenchmarkLogsWithoutFlatten(b *testing.B) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.LogStatements = []common.ContextStatements{
		{
			Context: "log",
			Statements: []string{
				`set(resource.attributes["host.name"], attributes["host.name"])`,
				`delete_key(attributes, "host.name")`,
			},
		},
	}
	sink := new(consumertest.LogsSink)
	p, err := factory.CreateLogs(b.Context(), processortest.NewNopSettings(metadata.Type), oCfg, sink)
	require.NoError(b, err)

	input, err := golden.ReadLogs(filepath.Join("testdata", "logs", "input.yaml"))
	require.NoError(b, err)

	for b.Loop() {
		require.NoError(b, p.ConsumeLogs(b.Context(), input))
	}
}

func BenchmarkLogsWithFlatten(b *testing.B) {
	benchmarks := []struct {
		name         string
		flattenLogs  bool
		groupFlatten bool
	}{
		{name: "flatten.logs", flattenLogs: true},
		{name: "flatten", groupFlatten: true},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)
			oCfg.Flatten.Logs = bm.flattenLogs
			oCfg.LogStatements = []common.ContextStatements{
				{
					Context: "log",
					Flatten: bm.groupFlatten,
					Statements: []string{
						`set(resource.attributes["host.name"], attributes["host.name"])`,
						`delete_key(attributes, "host.name")`,
					},
				},
			}
			sink := new(consumertest.LogsSink)
			p, err := factory.CreateLogs(b.Context(), processortest.NewNopSettings(metadata.Type), oCfg, sink)
			require.NoError(b, err)

			input, err := golden.ReadLogs(filepath.Join("testdata", "logs", "input.yaml"))
			require.NoError(b, err)

			for b.Loop() {
				require.NoError(b, p.ConsumeLogs(b.Context(), input))
			}
		})
	}
}
