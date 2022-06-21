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
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/attributes/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/internal/testutils"
)

const (
	testHostID                 = "example-host-id"
	testHostName               = "example-host-name"
	testContainerID            = "example-container-id"
	testClusterName            = "clusterName"
	testNodeName               = "nodeName"
	testCustomName             = "example-custom-host-name"
	testCloudAccount           = "projectID"
	testGCPHostname            = testHostName + ".c." + testCloudAccount + ".internal"
	testGCPIntegrationHostname = testHostName + "." + testCloudAccount
)

func TestHostnameFromAttributes(t *testing.T) {
	tests := []struct {
		name       string
		attrs      pcommon.Map
		usePreview bool

		ok       bool
		hostname string
	}{
		{
			name: "custom hostname",
			attrs: testutils.NewAttributeMap(map[string]string{
				AttributeDatadogHostname:            testCustomName,
				AttributeK8sNodeName:                testNodeName,
				conventions.AttributeK8SClusterName: testClusterName,
				conventions.AttributeContainerID:    testContainerID,
				conventions.AttributeHostID:         testHostID,
				conventions.AttributeHostName:       testHostName,
			}),
			ok:       true,
			hostname: testCustomName,
		},
		{
			name: "container ID",
			attrs: testutils.NewAttributeMap(map[string]string{
				conventions.AttributeContainerID: testContainerID,
			}),
			ok:       true,
			hostname: testContainerID,
		},
		{
			name: "container ID, preview",
			attrs: testutils.NewAttributeMap(map[string]string{
				conventions.AttributeContainerID: testContainerID,
			}),
			usePreview: true,
		},
		{
			name: "AWS EC2",
			attrs: testutils.NewAttributeMap(map[string]string{
				conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAWS,
				conventions.AttributeHostID:        testHostID,
				conventions.AttributeHostName:      testHostName,
			}),
			ok:       true,
			hostname: testHostName,
		},
		{
			name: "AWS EC2, preview",
			attrs: testutils.NewAttributeMap(map[string]string{
				conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAWS,
				conventions.AttributeHostID:        testHostID,
				conventions.AttributeHostName:      testHostName,
			}),
			ok:         true,
			hostname:   testHostID,
			usePreview: true,
		},
		{
			name: "ECS Fargate",
			attrs: testutils.NewAttributeMap(map[string]string{
				conventions.AttributeCloudProvider:      conventions.AttributeCloudProviderAWS,
				conventions.AttributeCloudPlatform:      conventions.AttributeCloudPlatformAWSECS,
				conventions.AttributeAWSECSTaskARN:      "example-task-ARN",
				conventions.AttributeAWSECSTaskFamily:   "example-task-family",
				conventions.AttributeAWSECSTaskRevision: "example-task-revision",
				conventions.AttributeAWSECSLaunchtype:   conventions.AttributeAWSECSLaunchtypeFargate,
			}),
			ok:       true,
			hostname: "",
		},
		{
			name: "GCP",
			attrs: testutils.NewAttributeMap(map[string]string{
				conventions.AttributeCloudProvider: conventions.AttributeCloudProviderGCP,
				conventions.AttributeHostID:        testHostID,
				conventions.AttributeHostName:      testGCPHostname,
			}),
			ok:       true,
			hostname: testGCPHostname,
		},
		{
			name: "GCP, preview",
			attrs: testutils.NewAttributeMap(map[string]string{
				conventions.AttributeCloudProvider:  conventions.AttributeCloudProviderGCP,
				conventions.AttributeHostID:         testHostID,
				conventions.AttributeHostName:       testGCPHostname,
				conventions.AttributeCloudAccountID: testCloudAccount,
			}),
			usePreview: true,
			ok:         true,
			hostname:   testGCPIntegrationHostname,
		},
		{
			name: "azure",
			attrs: testutils.NewAttributeMap(map[string]string{
				conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAzure,
				conventions.AttributeHostID:        testHostID,
				conventions.AttributeHostName:      testHostName,
			}),
			ok:       true,
			hostname: testHostName,
		},
		{
			name: "azure, preview",
			attrs: testutils.NewAttributeMap(map[string]string{
				conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAzure,
				conventions.AttributeHostID:        testHostID,
				conventions.AttributeHostName:      testHostName,
			}),
			usePreview: true,
			ok:         true,
			hostname:   testHostID,
		},
		{
			name: "host id v. hostname",
			attrs: testutils.NewAttributeMap(map[string]string{
				conventions.AttributeHostID:   testHostID,
				conventions.AttributeHostName: testHostName,
			}),
			ok:       true,
			hostname: testHostID,
		},
		{
			name:  "no hostname",
			attrs: testutils.NewAttributeMap(map[string]string{}),
		},
		{
			name: "localhost",
			attrs: testutils.NewAttributeMap(map[string]string{
				AttributeDatadogHostname: "127.0.0.1",
			}),
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			hostname, ok := HostnameFromAttributes(testInstance.attrs, testInstance.usePreview)
			assert.Equal(t, testInstance.ok, ok)
			assert.Equal(t, testInstance.hostname, hostname)
		})

	}
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
	hostname, ok := HostnameFromAttributes(attrs, false)
	assert.True(t, ok)
	assert.Equal(t, hostname, "nodeName-clusterName")

	// Node name, no cluster name
	attrs = testutils.NewAttributeMap(map[string]string{
		AttributeK8sNodeName:             testNodeName,
		conventions.AttributeContainerID: testContainerID,
		conventions.AttributeHostID:      testHostID,
		conventions.AttributeHostName:    testHostName,
	})
	hostname, ok = HostnameFromAttributes(attrs, false)
	assert.True(t, ok)
	assert.Equal(t, hostname, "nodeName")

	// Node name, no cluster name, AWS EC2, no preview
	attrs = testutils.NewAttributeMap(map[string]string{
		AttributeK8sNodeName:               testNodeName,
		conventions.AttributeContainerID:   testContainerID,
		conventions.AttributeHostID:        testHostID,
		conventions.AttributeHostName:      testHostName,
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAWS,
	})
	hostname, ok = HostnameFromAttributes(attrs, false)
	assert.True(t, ok)
	assert.Equal(t, hostname, "nodeName")

	// Node name, no cluster name, AWS EC2, preview
	attrs = testutils.NewAttributeMap(map[string]string{
		AttributeK8sNodeName:               testNodeName,
		conventions.AttributeContainerID:   testContainerID,
		conventions.AttributeHostID:        testHostID,
		conventions.AttributeHostName:      testHostName,
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAWS,
	})
	hostname, ok = HostnameFromAttributes(attrs, true)
	assert.True(t, ok)
	assert.Equal(t, hostname, testHostID)

	// no node name, cluster name
	attrs = testutils.NewAttributeMap(map[string]string{
		conventions.AttributeK8SClusterName: testClusterName,
		conventions.AttributeContainerID:    testContainerID,
		conventions.AttributeHostID:         testHostID,
		conventions.AttributeHostName:       testHostName,
	})
	hostname, ok = HostnameFromAttributes(attrs, false)
	assert.True(t, ok)
	// cluster name gets ignored, fallback to next option
	assert.Equal(t, hostname, testHostID)
}
