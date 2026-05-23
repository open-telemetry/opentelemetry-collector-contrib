// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unrollprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/unrollprocessor/internal/metadata"
)

func BenchmarkUnroll(b *testing.B) {
	unrollProcessor := &unrollProcessor{
		cfg: createDefaultConfig().(*Config),
	}
	testLogs := createTestResourceLogs()

	for b.Loop() {
		_, _ = unrollProcessor.ProcessLogs(b.Context(), testLogs)
	}
}

func createTestResourceLogs() plog.Logs {
	rl := plog.NewLogs()
	for range 10 {
		resourceLog := rl.ResourceLogs().AppendEmpty()
		for range 10 {
			scopeLogs := resourceLog.ScopeLogs().AppendEmpty()
			_ = scopeLogs.LogRecords().AppendEmpty().Body().SetEmptySlice().FromRaw([]any{1, 2, 3, 4, 5, 6, 7})
		}
	}
	return rl
}

func TestProcessor(t *testing.T) {
	for _, test := range []struct {
		name      string
		recursive bool
	}{
		{
			name: "nop",
		},
		{
			name: "simple",
		},
		{
			name: "mixed_slice_types",
		},
		{
			name: "some_not_slices",
		},
		{
			name: "recursive_false",
		},
		{
			name:      "recursive_true",
			recursive: true,
		},
		{
			name: "empty",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			input, err := golden.ReadLogs(filepath.Join("testdata", test.name, "input.yaml"))
			require.NoError(t, err)
			expected, err := golden.ReadLogs(filepath.Join("testdata", test.name, "expected.yaml"))
			require.NoError(t, err)

			f := NewFactory()
			cfg := f.CreateDefaultConfig().(*Config)
			cfg.Recursive = test.recursive
			set := processortest.NewNopSettings(metadata.Type)
			sink := &consumertest.LogsSink{}
			p, err := f.CreateLogs(t.Context(), set, cfg, sink)
			require.NoError(t, err)

			err = p.ConsumeLogs(t.Context(), input)
			require.NoError(t, err)

			actual := sink.AllLogs()
			require.Len(t, actual, 1)

			require.NoError(t, plogtest.CompareLogs(expected, actual[0]))
		})
	}
}
