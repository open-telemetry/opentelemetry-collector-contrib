// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestBatchLogs_ConsumeLogs(t *testing.T) {
	tests := []struct {
		name             string
		profilingEnabled bool
		logsEnabled      bool
		in               string
		out              []string
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
		},
		{
			name:             "combined_logs_disabled",
			profilingEnabled: true,
			in:               "combined.yaml",
			out:              []string{"profiling_only.yaml"},
		},
		{
			name: "combined_both_disabled",
			in:   "combined.yaml",
			out:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := &consumertest.LogsSink{}
			consumer := &perScopeBatcher{
				profilingEnabled: tt.profilingEnabled,
				logsEnabled:      tt.logsEnabled,
				next:             sink,
			}

			logs, err := golden.ReadLogs("testdata/batchperscope/" + tt.in)
			require.NoError(t, err)

			err = consumer.ConsumeLogs(context.Background(), logs)
			assert.NoError(t, err)

			assert.Equal(t, len(tt.out), len(sink.AllLogs()))
			for i, out := range tt.out {
				expected, err := golden.ReadLogs("testdata/batchperscope/" + out)
				require.NoError(t, err)
				assert.NoError(t, plogtest.CompareLogs(expected, sink.AllLogs()[i]))
			}
		})
	}

}
