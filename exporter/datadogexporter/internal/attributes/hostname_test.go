// Copyright  OpenTelemetry Authors
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

package attributes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
)

const (
	testHostID      = "example-host-id"
	testHostName    = "example-host-name"
	testContainerID = "example-container-id"
	testClusterName = "clusterName"
	testNodeName    = "nodeName"
	testCustomName  = "example-custom-host-name"
)

func TestHostnameFromAttributes(t *testing.T) {
	// Custom hostname
	attrs := testutils.NewAttributeMap(map[string]string{
		AttributeDatadogHostname:            testCustomName,
		AttributeK8sNodeName:                testNodeName,
		conventions.AttributeK8SClusterName: testClusterName,
		conventions.AttributeContainerID:    testContainerID,
		conventions.AttributeHostID:         testHostID,
		conventions.AttributeHostName:       testHostName,
	})
	hostname, ok := HostnameFromAttributes(attrs)
	assert.True(t, ok)
	assert.Equal(t, hostname, testCustomName)

	// Container ID
	attrs = testutils.NewAttributeMap(map[string]string{
		conventions.AttributeContainerID: testContainerID,
	})
	hostname, ok = HostnameFromAttributes(attrs)
	assert.True(t, ok)
	assert.Equal(t, hostname, testContainerID)

	// AWS cloud provider means relying on the EC2 function
	attrs = testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAWS,
		conventions.AttributeHostID:        testHostID,
		conventions.AttributeHostName:      testHostName,
	})
	hostname, ok = HostnameFromAttributes(attrs)
	assert.True(t, ok)
	assert.Equal(t, hostname, testHostName)

	// GCP cloud provider means relying on the GCP function
	attrs = testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderGCP,
		conventions.AttributeHostID:        testHostID,
		conventions.AttributeHostName:      testHostName,
	})
	hostname, ok = HostnameFromAttributes(attrs)
	assert.True(t, ok)
	assert.Equal(t, hostname, testHostName)

	// Azure cloud provider means relying on the Azure function
	attrs = testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAzure,
		conventions.AttributeHostID:        testHostID,
		conventions.AttributeHostName:      testHostName,
	})
	hostname, ok = HostnameFromAttributes(attrs)
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

func TestGetClusterName(t *testing.T) {
	// OpenTelemetry convention
	attrs := testutils.NewAttributeMap(map[string]string{
		conventions.AttributeK8SClusterName: testClusterName,
	})
	cluster, ok := getClusterName(attrs)
	assert.True(t, ok)
	assert.Equal(t, cluster, testClusterName)

	// Azure
	attrs = testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAzure,
		azure.AttributeResourceGroupName:   "MC_aks-kenafeh_aks-kenafeh-eu_westeurope",
	})
	cluster, ok = getClusterName(attrs)
	assert.True(t, ok)
	assert.Equal(t, cluster, "aks-kenafeh-eu")

	// AWS
	attrs = testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider:          conventions.AttributeCloudProviderAWS,
		"ec2.tag.kubernetes.io/cluster/clustername": "dummy_value",
	})
	cluster, ok = getClusterName(attrs)
	assert.True(t, ok)
	assert.Equal(t, cluster, "clustername")

	// None
	attrs = testutils.NewAttributeMap(map[string]string{})
	_, ok = getClusterName(attrs)
	assert.False(t, ok)
}

func TestHostnameKubernetes(t *testing.T) {
	// Node name and cluster name
	attrs := testutils.NewAttributeMap(map[string]string{
		AttributeK8sNodeName:                testNodeName,
		conventions.AttributeK8SClusterName: testClusterName,
		conventions.AttributeContainerID:    testContainerID,
		conventions.AttributeHostID:         testHostID,
		conventions.AttributeHostName:       testHostName,
	})
	hostname, ok := HostnameFromAttributes(attrs)
	assert.True(t, ok)
	assert.Equal(t, hostname, "nodeName-clusterName")

	// Node name, no cluster name
	attrs = testutils.NewAttributeMap(map[string]string{
		AttributeK8sNodeName:             testNodeName,
		conventions.AttributeContainerID: testContainerID,
		conventions.AttributeHostID:      testHostID,
		conventions.AttributeHostName:    testHostName,
	})
	hostname, ok = HostnameFromAttributes(attrs)
	assert.True(t, ok)
	assert.Equal(t, hostname, "nodeName")

	// no node name, cluster name
	attrs = testutils.NewAttributeMap(map[string]string{
		conventions.AttributeK8SClusterName: testClusterName,
		conventions.AttributeContainerID:    testContainerID,
		conventions.AttributeHostID:         testHostID,
		conventions.AttributeHostName:       testHostName,
	})
	hostname, ok = HostnameFromAttributes(attrs)
	assert.True(t, ok)
	// cluster name gets ignored, fallback to next option
	assert.Equal(t, hostname, testHostID)
}
