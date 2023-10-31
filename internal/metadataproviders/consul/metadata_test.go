// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
