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
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata/gcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata/system"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata/valid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/utils/cache"
)

const (
	AttributeDatadogHostname = "datadog.host.name"
	AttributeK8sNodeName     = "k8s.node.name"
)

// GetHost gets the hostname according to configuration.
// It checks in the following order
// 1. Configuration
// 2. Cache
// 3. EC2 instance metadata
// 4. System
func GetHost(logger *zap.Logger, cfg *config.Config) *string {
	if cfg.Hostname != "" {
		return &cfg.Hostname
	}

	if cacheVal, ok := cache.Cache.Get(cache.CanonicalHostnameKey); ok {
		return cacheVal.(*string)
	}

	ec2Info := ec2.GetHostInfo(logger)
	hostname := ec2Info.GetHostname(logger)

	if hostname == "" {
		// Get system hostname
		systemInfo := system.GetHostInfo(logger)
		hostname = systemInfo.GetHostname(logger)
	}

	if err := valid.Hostname(hostname); err != nil {
		// If invalid log but continue
		logger.Error("Detected hostname is not valid", zap.Error(err))
	}

	logger.Debug("Canonical hostname automatically set", zap.String("hostname", hostname))
	cache.Cache.Set(cache.CanonicalHostnameKey, &hostname, cache.NoExpiration)
	return &hostname
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
		if k8sClusterName, ok := attrs.Get(conventions.AttributeK8sCluster); ok {
			return k8sNodeName.StringVal() + "-" + k8sClusterName.StringVal(), true
		}
		return k8sNodeName.StringVal(), true
	}

	// container id (e.g. from Docker)
	if containerID, ok := attrs.Get(conventions.AttributeContainerID); ok {
		return containerID.StringVal(), true
	}

	// handle AWS case separately to have similar behavior to the Datadog Agent
	cloudProvider, ok := attrs.Get(conventions.AttributeCloudProvider)
	if ok && cloudProvider.StringVal() == conventions.AttributeCloudProviderAWS {
		return ec2.HostnameFromAttributes(attrs)
	} else if ok && cloudProvider.StringVal() == conventions.AttributeCloudProviderGCP {
		return gcp.HostnameFromAttributes(attrs)
	}

	// host id from cloud provider
	if hostID, ok := attrs.Get(conventions.AttributeHostID); ok {
		return hostID.StringVal(), true
	}

	// hostname from cloud provider or OS
	if hostName, ok := attrs.Get(conventions.AttributeHostName); ok {
		return hostName.StringVal(), true
	}

	return "", false
}
