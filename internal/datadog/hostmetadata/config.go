// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata"

import (
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
)

// PusherConfig is the configuration for the metadata pusher goroutine.
type PusherConfig struct {
	// ConfigHosthame is the hostname set in the configuration of the exporter (empty if unset).
	ConfigHostname string
	// ConfigTags are the tags set in the configuration of the exporter (empty if unset).
	ConfigTags []string
	// MetricsEndpoint is the metrics endpoint.
	MetricsEndpoint string
	// APIKey is the API key set in configuration.
	APIKey string
	// UseResourceMetadata is the value of 'use_resource_metadata' on the top-level configuration.
	UseResourceMetadata bool
	// InsecureSkipVerify is the value of `tls.insecure_skip_verify` on the configuration.
	InsecureSkipVerify bool
	// ClientConfig of exporter.
	ClientConfig confighttp.ClientConfig
	// RetrySettings of exporter.
	RetrySettings configretry.BackOffConfig
	// ReporterPeriod is the period of the reporter goroutine.
	ReporterPeriod time.Duration
}
