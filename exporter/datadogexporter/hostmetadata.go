// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata"
)

// newMetadataConfigfromConfig creates a new metadata pusher config from the main
func newMetadataConfigfromConfig(cfg *Config) hostmetadata.PusherConfig {
	return hostmetadata.PusherConfig{
		ConfigHostname:      cfg.Hostname,
		ConfigTags:          cfg.HostMetadata.Tags,
		MetricsEndpoint:     cfg.Metrics.Endpoint,
		APIKey:              string(cfg.API.Key),
		UseResourceMetadata: cfg.HostMetadata.HostnameSource == HostnameSourceFirstResource,
		InsecureSkipVerify:  cfg.TLSSetting.InsecureSkipVerify,
		TimeoutSettings:     cfg.TimeoutSettings,
		RetrySettings:       cfg.BackOffConfig,
	}
}
