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
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/utils/cache"
)

func TestHost(t *testing.T) {

	logger := zap.NewNop()

	// Start with a fresh cache, the following test would fail
	// if the cache key is already set.
	cache.Cache.Delete(cache.CanonicalHostnameKey)

	host := GetHost(logger, &config.Config{
		TagsConfig: config.TagsConfig{Hostname: "test-host"},
	})
	assert.Equal(t, host, "test-host")

	// config.Config.Hostname does not get stored in the cache
	host = GetHost(logger, &config.Config{
		TagsConfig: config.TagsConfig{Hostname: "test-host-2"},
	})
	assert.Equal(t, host, "test-host-2")

	// Disable EC2 Metadata service to prevent fetching hostname from there,
	// in case the test is running on an EC2 instance
	const awsEc2MetadataDisabled = "AWS_EC2_METADATA_DISABLED"
	curr := os.Getenv(awsEc2MetadataDisabled)
	err := os.Setenv(awsEc2MetadataDisabled, "true")
	require.NoError(t, err)
	defer os.Setenv(awsEc2MetadataDisabled, curr)

	host = GetHost(logger, &config.Config{})
	osHostname, err := os.Hostname()
	require.NoError(t, err)
	// TODO: Investigate why the returned host contains more data on github actions.
	assert.Contains(t, host, osHostname)
}

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
		AttributeDatadogHostname:         testCustomName,
		AttributeK8sNodeName:             testNodeName,
		conventions.AttributeK8sCluster:  testClusterName,
		conventions.AttributeContainerID: testContainerID,
		conventions.AttributeHostID:      testHostID,
		conventions.AttributeHostName:    testHostName,
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
		conventions.AttributeK8sCluster: testClusterName,
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
		AttributeK8sNodeName:             testNodeName,
		conventions.AttributeK8sCluster:  testClusterName,
		conventions.AttributeContainerID: testContainerID,
		conventions.AttributeHostID:      testHostID,
		conventions.AttributeHostName:    testHostName,
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
		conventions.AttributeK8sCluster:  testClusterName,
		conventions.AttributeContainerID: testContainerID,
		conventions.AttributeHostID:      testHostID,
		conventions.AttributeHostName:    testHostName,
	})
	hostname, ok = HostnameFromAttributes(attrs)
	assert.True(t, ok)
	// cluster name gets ignored, fallback to next option
	assert.Equal(t, hostname, testHostID)
}
