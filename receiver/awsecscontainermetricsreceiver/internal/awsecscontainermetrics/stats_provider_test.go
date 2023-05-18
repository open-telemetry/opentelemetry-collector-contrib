// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetrics

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil/ecsutiltest"
)

type testRestClient struct {
	*testing.T
	fail        bool
	invalidJSON bool
}

func (f testRestClient) GetResponse(path string) ([]byte, error) {
	if body, err := ecsutiltest.GetTestdataResponseByPath(f.T, path); body != nil || err != nil {
		return body, err
	}

	if f.fail {
		return []byte{}, fmt.Errorf("failed")
	}
	if f.invalidJSON {
		return []byte("wrong-json-body"), nil
	}

	if path == TaskStatsPath {
		return os.ReadFile("../../testdata/task_stats.json")
	}

	return nil, nil
}

func TestGetStats(t *testing.T) {
	tests := []struct {
		name      string
		client    ecsutil.RestClient
		wantError string
	}{
		{
			name:      "success",
			client:    &testRestClient{},
			wantError: "",
		},
		{
			name:      "failure",
			client:    &testRestClient{fail: true},
			wantError: "cannot read data from task metadata endpoint: failed",
		},
		{
			name:      "invalid-json",
			client:    &testRestClient{invalidJSON: true},
			wantError: "cannot unmarshall task stats: invalid character 'w' looking for beginning of value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewStatsProvider(tt.client, zap.NewNop())
			stats, metadata, err := provider.GetStats()
			if tt.wantError == "" {
				require.NoError(t, err)
				require.Less(t, 0, len(stats))
				require.Equal(t, "test200", metadata.Cluster)
			} else {
				assert.Equal(t, tt.wantError, err.Error())
			}
		})
	}
}
