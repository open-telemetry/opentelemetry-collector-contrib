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
package azure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
)

var (
	testVMID     = "02aab8a4-74ef-476e-8182-f6d2ba4166a6"
	testHostname = "test-hostname"
	testAttrs    = testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider:  conventions.AttributeCloudProviderAzure,
		conventions.AttributeHostName:       testHostname,
		conventions.AttributeCloudRegion:    "location",
		conventions.AttributeHostID:         testVMID,
		conventions.AttributeCloudAccountID: "subscriptionID",
		AttributeResourceGroupName:          "resourceGroup",
	})
	testEmpty = testutils.NewAttributeMap(map[string]string{})
)

func TestHostInfoFromAttributes(t *testing.T) {
	hostInfo := HostInfoFromAttributes(testAttrs)
	require.NotNil(t, hostInfo)
	assert.ElementsMatch(t, hostInfo.HostAliases, []string{testVMID})

	emptyHostInfo := HostInfoFromAttributes(testEmpty)
	require.NotNil(t, emptyHostInfo)
	assert.ElementsMatch(t, emptyHostInfo.HostAliases, []string{})
}

func TestHostnameFromAttributes(t *testing.T) {
	hostname, ok := HostnameFromAttributes(testAttrs)
	assert.True(t, ok)
	assert.Equal(t, hostname, testHostname)

	_, ok = HostnameFromAttributes(testEmpty)
	assert.False(t, ok)
}

func TestClusterNameFromAttributes(t *testing.T) {
	cluster, ok := ClusterNameFromAttributes(testutils.NewAttributeMap(map[string]string{
		AttributeResourceGroupName: "MC_aks-kenafeh_aks-kenafeh-eu_westeurope",
	}))
	assert.True(t, ok)
	assert.Equal(t, cluster, "aks-kenafeh-eu")

	cluster, ok = ClusterNameFromAttributes(testutils.NewAttributeMap(map[string]string{
		AttributeResourceGroupName: "mc_foo-bar-aks-k8s-rg_foo-bar-aks-k8s_westeurope",
	}))
	assert.True(t, ok)
	assert.Equal(t, cluster, "foo-bar-aks-k8s")

	_, ok = ClusterNameFromAttributes(testutils.NewAttributeMap(map[string]string{
		AttributeResourceGroupName: "unexpected-resource-group-name-format",
	}))
	assert.False(t, ok)

	_, ok = ClusterNameFromAttributes(testutils.NewAttributeMap(map[string]string{}))
	assert.False(t, ok)
}
