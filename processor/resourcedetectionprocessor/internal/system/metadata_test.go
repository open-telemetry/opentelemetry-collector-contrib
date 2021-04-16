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

package system

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

func TestGOOSToOsType(t *testing.T) {
	assert.Equal(t, "DARWIN", goosToOSType("darwin"))
	assert.Equal(t, "LINUX", goosToOSType("linux"))
	assert.Equal(t, "WINDOWS", goosToOSType("windows"))
	assert.Equal(t, "DRAGONFLYBSD", goosToOSType("dragonfly"))
}

func TestDockerError(t *testing.T) {
	_, err := newDockerMetadata(client.WithHost("invalidHost"))
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

	provider, err := newDockerMetadata(client.WithHost(ts.URL))
	require.NoError(t, err)

	_, err = provider.FQDN(context.Background())
	assert.Error(t, err)

	hostname, err := provider.Hostname(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "hostname", hostname)

	osType, err := provider.OSType(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "LINUX", osType)
}
