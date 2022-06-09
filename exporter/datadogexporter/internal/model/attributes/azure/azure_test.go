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
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/internal/testutils"
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
	tests := []struct {
		name       string
		attrs      pcommon.Map
		usePreview bool

		ok       bool
		hostname string
		aliases  []string
	}{
		{
			name:  "all attributes",
			attrs: testAttrs,

			ok:       true,
			hostname: testHostname,
			aliases:  []string{testVMID},
		},
		{
			name:  "empty",
			attrs: testEmpty,
		},
		{
			name:       "all attributes with preview",
			attrs:      testAttrs,
			usePreview: true,

			ok:       true,
			hostname: testVMID,
		},
		{
			name:       "empty with preview",
			attrs:      testEmpty,
			usePreview: true,
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			hostInfo := HostInfoFromAttributes(testInstance.attrs, testInstance.usePreview)
			require.NotNil(t, hostInfo)
			assert.ElementsMatch(t, testInstance.aliases, hostInfo.HostAliases)
			hostname, ok := HostnameFromAttributes(testInstance.attrs, testInstance.usePreview)

			assert.Equal(t, testInstance.ok, ok)
			if testInstance.ok || ok {
				assert.Equal(t, testInstance.hostname, hostname)
			}
		})
	}
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
