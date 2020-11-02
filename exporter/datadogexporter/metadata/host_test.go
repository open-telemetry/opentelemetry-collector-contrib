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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/utils/cache"
)

func TestHost(t *testing.T) {

	logger := zap.NewNop()

	// Start with a fresh cache, the following test would fail
	// if the cache key is already set.
	cache.Cache.Delete(cache.CanonicalHostnameKey)

	host := GetHost(logger, &config.Config{
		TagsConfig: config.TagsConfig{Hostname: "test-host"},
	})
	assert.Equal(t, *host, "test-host")
	cache.Cache.Delete(cache.CanonicalHostnameKey)

	host = GetHost(logger, &config.Config{})
	osHostname, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t, *host, osHostname)
}

func TestHostnameFromAttributes(t *testing.T) {
	testHostID := "example-host-id"
	testHostName := "example-host-name"

	// AWS cloud provider means relying on the EC2 function
	attrs := testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAWS,
		conventions.AttributeHostID:        testHostID,
		conventions.AttributeHostName:      testHostName,
	})
	hostname, ok := HostnameFromAttributes(attrs)
	assert.True(t, ok)
	assert.Equal(t, hostname, testHostName)

	// Host Id takes preference
	attrs = testutils.NewAttributeMap(map[string]string{
		conventions.AttributeHostID:   testHostID,
		conventions.AttributeHostName: testHostName,
	})
	hostname, ok = HostnameFromAttributes(attrs)
	assert.True(t, ok)
	assert.Equal(t, hostname, testHostID)

	// No labels means no hostname
	attrs = testutils.NewAttributeMap(map[string]string{})
	hostname, ok = HostnameFromAttributes(attrs)
	assert.False(t, ok)
	assert.Empty(t, hostname)

}
