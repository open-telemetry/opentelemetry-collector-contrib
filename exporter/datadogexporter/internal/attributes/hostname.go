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
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes/gcp"
)

const (
	// AttributeDatadogHostname the datadog host name attribute
	AttributeDatadogHostname = "datadog.host.name"
	// AttributeK8sNodeName the datadog k8s node name attribute
	AttributeK8sNodeName = "k8s.node.name"
)

func getClusterName(attrs pdata.AttributeMap) (string, bool) {
	if k8sClusterName, ok := attrs.Get(conventions.AttributeK8SClusterName); ok {
		return k8sClusterName.StringVal(), true
	}

	cloudProvider, ok := attrs.Get(conventions.AttributeCloudProvider)
	if ok && cloudProvider.StringVal() == conventions.AttributeCloudProviderAzure {
		return azure.ClusterNameFromAttributes(attrs)
	} else if ok && cloudProvider.StringVal() == conventions.AttributeCloudProviderAWS {
		return ec2.ClusterNameFromAttributes(attrs)
	}

	return "", false
}

// HostnameFromAttributes tries to get a valid hostname from attributes by checking, in order:
//
//   1. a custom Datadog hostname provided by the "datadog.host.name" attribute
//   2. the Kubernetes node name (and cluster name if available),
//   3. cloud provider specific hostname for AWS or GCP
//   4. the container ID,
//   5. the cloud provider host ID and
//   6. the host.name attribute.
//
//  It returns a boolean value indicated if any name was found
func HostnameFromAttributes(attrs pdata.AttributeMap) (string, bool) {
	// Custom hostname: useful for overriding in k8s/cloud envs
	if customHostname, ok := attrs.Get(AttributeDatadogHostname); ok {
		return customHostname.StringVal(), true
	}

	// Kubernetes: node-cluster if cluster name is available, else node
	if k8sNodeName, ok := attrs.Get(AttributeK8sNodeName); ok {
		if k8sClusterName, ok := getClusterName(attrs); ok {
			return k8sNodeName.StringVal() + "-" + k8sClusterName, true
		}
		return k8sNodeName.StringVal(), true
	}

	cloudProvider, ok := attrs.Get(conventions.AttributeCloudProvider)
	if ok && cloudProvider.StringVal() == conventions.AttributeCloudProviderAWS {
		return ec2.HostnameFromAttributes(attrs)
	} else if ok && cloudProvider.StringVal() == conventions.AttributeCloudProviderGCP {
		return gcp.HostnameFromAttributes(attrs)
	} else if ok && cloudProvider.StringVal() == conventions.AttributeCloudProviderAzure {
		return azure.HostnameFromAttributes(attrs)
	}

	// host id from cloud provider
	if hostID, ok := attrs.Get(conventions.AttributeHostID); ok {
		return hostID.StringVal(), true
	}

	// hostname from cloud provider or OS
	if hostName, ok := attrs.Get(conventions.AttributeHostName); ok {
		return hostName.StringVal(), true
	}

	// container id (e.g. from Docker)
	if containerID, ok := attrs.Get(conventions.AttributeContainerID); ok {
		return containerID.StringVal(), true
	}

	return "", false
}
