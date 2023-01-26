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

package metadata

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestHost(t *testing.T) {
	// Start with a fresh cache, the following test would fail
	// if the cache key is already set.
	hostnameCache.Delete(cacheKeyHostname)

	p, err := buildCurrentProvider(componenttest.NewNopTelemetrySettings(), "test-host")
	require.NoError(t, err)
	src, err := p.Source(context.Background())
	require.NoError(t, err)
	assert.Equal(t, src.Identifier, "test-host")

	// config.Config.Hostname does not get stored in the cache
	p, err = buildCurrentProvider(componenttest.NewNopTelemetrySettings(), "test-host-2")
	require.NoError(t, err)
	src, err = p.Source(context.Background())
	require.NoError(t, err)
	assert.Equal(t, src.Identifier, "test-host-2")

	// Disable EC2 Metadata service to prevent fetching hostname from there,
	// in case the test is running on an EC2 instance
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	p, err = buildCurrentProvider(componenttest.NewNopTelemetrySettings(), "")
	require.NoError(t, err)
	src, err = p.Source(context.Background())
	require.NoError(t, err)
	osHostname, err := os.Hostname()
	require.NoError(t, err)
	assert.Contains(t, src.Identifier, osHostname)
}
