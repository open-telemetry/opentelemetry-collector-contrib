// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsecscontainermetrics

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRestClient struct {
	fail                bool
	invalidJSON         bool
	invalidTaskMetadata bool
}

func (f testRestClient) EndpointResponse() ([]byte, []byte, error) {
	if f.fail {
		return []byte{}, []byte{}, fmt.Errorf("failed")
	}
	if f.invalidJSON {
		return []byte("wrong-json-body"), []byte("wrong-json-body"), nil
	}

	taskStats, _ := ioutil.ReadFile("../../testdata/task_stats.json")
	if f.invalidTaskMetadata {
		return taskStats, []byte("wrong-json-body"), nil
	}

	taskStats, err := ioutil.ReadFile("../../testdata/task_stats.json")
	if err != nil {
		return nil, nil, err
	}
	taskMetadata, err := ioutil.ReadFile("../../testdata/task_metadata.json")
	if err != nil {
		return nil, nil, err
	}
	return taskStats, taskMetadata, nil
}

func TestGetStats(t *testing.T) {
	tests := []struct {
		name      string
		client    RestClient
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
		{
			name:      "invalid-task-metadata",
			client:    &testRestClient{invalidTaskMetadata: true},
			wantError: "cannot unmarshall task metadata: invalid character 'w' looking for beginning of value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewStatsProvider(tt.client)
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
