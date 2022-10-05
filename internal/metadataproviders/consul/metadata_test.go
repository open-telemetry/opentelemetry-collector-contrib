// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package consul

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsulHappyPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() == "/v1/agent/self" {
			fmt.Fprintln(w, `{
            "Config": {
                "Datacenter":"dc1",
                "NodeName":"hostname",
                "NodeID": "00000000-0000-0000-0000-000000000000"
              },
            "Meta": {
                "test": "test",
                "environment": "prod"
            }
        }`)
		}
	}))
	defer ts.Close()

	config := api.DefaultConfig()
	config.Address = ts.URL

	allowedLabels := map[string]interface{}{
		"test": nil,
	}
	client, err := api.NewClient(config)
	require.NoError(t, err)
	provider := NewProvider(client, allowedLabels)
	meta, err := provider.Metadata(context.TODO())
	assert.NoError(t, err)
	assert.Equal(t, "hostname", meta.Hostname)
	assert.Equal(t, "dc1", meta.Datacenter)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", meta.NodeID)
	assert.Equal(t, map[string]string{"test": "test"}, meta.HostMetadata)
}
