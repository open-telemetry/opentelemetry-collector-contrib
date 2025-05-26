// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

// newMetadataConfigfromConfig creates a new metadata pusher config from the main
func newMetadataConfigfromConfig(cfg *datadogconfig.Config) hostmetadata.PusherConfig {
	return hostmetadata.PusherConfig{
		ConfigHostname:      cfg.Hostname,
		ConfigTags:          cfg.HostMetadata.Tags,
		MetricsEndpoint:     cfg.Metrics.Endpoint,
		APIKey:              string(cfg.API.Key),
		UseResourceMetadata: cfg.HostMetadata.HostnameSource == datadogconfig.HostnameSourceFirstResource,
		InsecureSkipVerify:  cfg.TLSSetting.InsecureSkipVerify,
		ClientConfig:        cfg.ClientConfig,
		RetrySettings:       cfg.BackOffConfig,
		ReporterPeriod:      cfg.HostMetadata.ReporterPeriod,
	}
}
