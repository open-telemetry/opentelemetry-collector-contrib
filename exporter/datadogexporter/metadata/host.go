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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata/system"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata/valid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/utils/cache"
)

// GetHost gets the hostname according to configuration.
// It gets the configuration hostname and if
// not available it relies on the OS hostname
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

	// Get system hostname
	hostInfo := system.GetHostInfo(logger)
	hostname := hostInfo.GetHostname(logger)

	if err := valid.Hostname(hostname); err != nil {
		// If invalid log but continue
		logger.Error("Detected hostname is not valid", zap.Error(err))
	}

	logger.Debug("Canonical hostname automatically set", zap.String("hostname", hostname))
	cache.Cache.Set(cache.CanonicalHostnameKey, &hostname, cache.NoExpiration)
	return &hostname
}
