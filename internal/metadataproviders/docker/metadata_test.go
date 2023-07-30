// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerError(t *testing.T) {
	_, err := NewProvider(client.WithHost("invalidHost"))
	assert.Error(t, err)
}

func TestDocker(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{
			"OSType":"linux",
			"Name":"hostname"
		}`)
	}))
	defer ts.Close()

	provider, err := NewProvider(client.WithHost(ts.URL))
	require.NoError(t, err)

	hostname, err := provider.Hostname(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "hostname", hostname)

	osType, err := provider.OSType(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "linux", osType)
}
