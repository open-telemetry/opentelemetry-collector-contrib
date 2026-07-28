// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dnscheckreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dnscheckreceiver/internal/metadata"
)

var (
	errMissingDNSServers = errors.New("at least one 'dns_servers' entry must be specified")
	errMissingHostnames  = errors.New("at least one 'hostnames' entry must be specified")
	errMissingHostname   = errors.New("hostname 'name' is required")
)

// Config defines the configuration for the DNS check receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`

	// DNSServers is the list of DNS servers to query, as "host" or "host:port" (default port 53).
	DNSServers []string `mapstructure:"dns_servers"`

	// Hostnames is the list of hostnames/record types to resolve against each DNS server.
	Hostnames []HostnameConfig `mapstructure:"hostnames"`

	// Timeout is the per-query dial timeout.
	Timeout time.Duration `mapstructure:"timeout"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// HostnameConfig defines a single hostname and record type to resolve.
type HostnameConfig struct {
	Name       string `mapstructure:"name"`
	RecordType string `mapstructure:"record_type"`
}

func (c *Config) Validate() error {
	var err error

	if len(c.DNSServers) == 0 {
		err = multierr.Append(err, errMissingDNSServers)
	}

	if len(c.Hostnames) == 0 {
		err = multierr.Append(err, errMissingHostnames)
	}

	for _, hostname := range c.Hostnames {
		if hostname.Name == "" {
			err = multierr.Append(err, errMissingHostname)
		}
	}

	return err
}
