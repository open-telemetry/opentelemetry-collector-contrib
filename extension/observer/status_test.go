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

package observer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatus_json(t *testing.T) {
	status := &Status{state: map[string]Endpoint{
		"id-1": {
			ID:     "id-1",
			Target: "1.2.3.4:8080",
			Details: Pod{
				Name: "pod-1",
				Labels: map[string]string{
					"label-1": "value-1",
				},
			},
		},
	}}

	data, err := status.json()
	require.NoError(t, err)
	assert.Equal(t, `[{"id":"id-1","target":"1.2.3.4:8080","details":{"name":"pod-1","labels":{"label-1":"value-1"}},"type":"Pod"}]`, string(data))
}
