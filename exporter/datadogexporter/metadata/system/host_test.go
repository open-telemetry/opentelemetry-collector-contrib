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

package system

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/utils/cache"
)

func TestGetHostInfo(t *testing.T) {
	logger := zap.NewNop()
	testHostInfo := &HostInfo{OS: "test-os", FQDN: "test-fqdn"}

	// Check it is retrieved from cache
	cache.SetNoExpire(cache.SystemHostInfoKey, testHostInfo)
	assert.Equal(t, testHostInfo, GetHostInfo(logger))
	cache.Delete(cache.SystemHostInfoKey)

	hostInfo := GetHostInfo(logger)
	require.NotNil(t, hostInfo)

	osHostname, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t, hostInfo.OS, osHostname)
}
