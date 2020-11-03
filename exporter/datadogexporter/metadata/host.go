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
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata/system"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata/valid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/utils/cache"
)

// GetHost gets the hostname according to configuration.
// It checks in the following order
// 1. Cache
// 2. Configuration
// 3. EC2 instance metadata
// 4. System
func GetHost(logger *zap.Logger, cfg *config.Config) *string {
	if cacheVal, ok := cache.Cache.Get(cache.CanonicalHostnameKey); ok {
		return cacheVal.(*string)
	}

	if err := valid.Hostname(cfg.Hostname); err == nil {
		cache.Cache.Add(cache.CanonicalHostnameKey, &cfg.Hostname, cache.NoExpiration)
		return &cfg.Hostname
	} else if cfg.Hostname != "" {
		logger.Error("Hostname set in configuration is invalid", zap.Error(err))
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
//   1. the container ID,
//   2. the cloud provider host ID and
//   3. the host.name attribute.
//
//  It returns a boolean value indicated if any name was found
func HostnameFromAttributes(attrs pdata.AttributeMap) (string, bool) {
	if containerID, ok := attrs.Get(conventions.AttributeContainerID); ok {
		return containerID.StringVal(), true
	}

	// handle AWS case separately to have similar behavior to the Datadog Agent
	cloudProvider, ok := attrs.Get(conventions.AttributeCloudProvider)
	if ok && cloudProvider.StringVal() == conventions.AttributeCloudProviderAWS {
		return ec2.HostnameFromAttributes(attrs)
	}

	if hostID, ok := attrs.Get(conventions.AttributeHostID); ok {
		return hostID.StringVal(), true
	}

	if hostName, ok := attrs.Get(conventions.AttributeHostName); ok {
		return hostName.StringVal(), true
	}

	return "", false
}
