// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package networkscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper/internal/metadata"
)

// Config relating to Network Metric Scraper.
type Config struct {
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	// Include specifies a filter on the network interfaces that should be included from the generated metrics.
	Include MatchConfig `mapstructure:"include"`
	// Exclude specifies a filter on the network interfaces that should be excluded from the generated metrics.
	Exclude MatchConfig `mapstructure:"exclude"`
	// Connections specifies detailed network connection metric settings.
	Connections ConnectionConfig `mapstructure:"connections"`
}

type MatchConfig struct {
	filterset.Config `mapstructure:",squash"`

	Interfaces []string `mapstructure:"interfaces"`
}

type ConnectionConfig struct {
	// IncludePorts specifies remote ports that should be included in detailed connection metrics.
	IncludePorts []uint32 `mapstructure:"include_ports"`
	// ExcludePorts specifies remote ports that should be excluded from detailed connection metrics.
	ExcludePorts []uint32 `mapstructure:"exclude_ports"`
	// ExcludeLocalhost specifies whether to exclude local and loopback remote addresses from detailed connection metrics.
	ExcludeLocalhost bool `mapstructure:"exclude_localhost"`
	// ExcludeListenPorts specifies whether to exclude connections from local listening ports from detailed connection metrics.
	ExcludeListenPorts bool `mapstructure:"exclude_listen_ports"`
}
