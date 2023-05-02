// Copyright The OpenTelemetry Authors
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

package kubelet

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRestClient struct {
	fail        bool
	invalidJSON bool
}

func (f testRestClient) StatsSummary() ([]byte, error) {
	return []byte{}, nil
}

func (f testRestClient) Pods() ([]byte, error) {
	if f.fail {
		return []byte{}, errors.New("failed")
	}
	if f.invalidJSON {
		return []byte("wrong-json-body"), nil
	}

	return os.ReadFile("../../testdata/pods.json")
}

func TestPods(t *testing.T) {
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
			wantError: "failed",
		},
		{
			name:      "invalid-json",
			client:    &testRestClient{invalidJSON: true},
			wantError: "invalid character 'w' looking for beginning of value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadataProvider := NewMetadataProvider(tt.client)
			podsMetadata, err := metadataProvider.Pods()
			if tt.wantError == "" {
				require.NoError(t, err)
				require.Less(t, 0, len(podsMetadata.Items))
			} else {
				assert.Equal(t, tt.wantError, err.Error())
			}
		})
	}
}
