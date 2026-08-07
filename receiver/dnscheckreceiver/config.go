// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dnscheckreceiver"

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dnscheckreceiver/internal/metadata"
)

// validNetworks are the transport protocols supported when querying a DNS server,
// matching the network values accepted by github.com/miekg/dns's Client.Net.
var validNetworks = map[string]struct{}{
	"udp":     {},
	"tcp":     {},
	"tcp-tls": {},
}

var (
	errMissingDNSServers       = errors.New("at least one 'dns_servers' entry must be specified")
	errMissingDNSServer        = errors.New("dns server 'endpoint' is required")
	errInvalidDNSServerNetwork = errors.New("dns server 'network' must be one of udp, tcp, tcp-tls")
	errMissingHostnames        = errors.New("at least one 'hostnames' entry must be specified")
	errMissingHostname         = errors.New("hostname 'name' is required")
	errInvalidRecordType       = errors.New("hostname 'record_type' must be a valid DNS record type")
)

// Config defines the configuration for the DNS check receiver.
type Config struct {
	ControllerConfig     scraperhelper.ControllerConfig `mapstructure:",squash"`
	MetricsBuilderConfig metadata.MetricsBuilderConfig  `mapstructure:",squash"`

	// DNSServers is the list of DNS servers to query.
	DNSServers []DNSServerConfig `mapstructure:"dns_servers"`

	// Hostnames is the list of hostnames/record types to resolve against each DNS server.
	Hostnames []HostnameConfig `mapstructure:"hostnames"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// DNSServerConfig defines a single DNS server to query.
type DNSServerConfig struct {
	// Endpoint is the DNS server address, as "host" or "host:port" (default port 53).
	Endpoint string `mapstructure:"endpoint"`

	// Network is the transport protocol used to query the server: "udp", "tcp", or "tcp-tls".
	// Defaults to "udp".
	Network string `mapstructure:"network"`

	// Timeout is the per-query dial timeout for this server. Defaults to 5s.
	Timeout time.Duration `mapstructure:"timeout"`
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

	for _, dnsServer := range c.DNSServers {
		if dnsServer.Endpoint == "" {
			err = multierr.Append(err, errMissingDNSServer)
		}

		if dnsServer.Network != "" {
			if _, ok := validNetworks[dnsServer.Network]; !ok {
				err = multierr.Append(err, fmt.Errorf("%w: got %q for endpoint %q", errInvalidDNSServerNetwork, dnsServer.Network, dnsServer.Endpoint))
			}
		}
	}

	if len(c.Hostnames) == 0 {
		err = multierr.Append(err, errMissingHostnames)
	}

	for _, hostname := range c.Hostnames {
		if hostname.Name == "" {
			err = multierr.Append(err, errMissingHostname)
		}

		if hostname.RecordType != "" {
			if _, ok := dns.StringToType[strings.ToUpper(hostname.RecordType)]; !ok {
				err = multierr.Append(err, fmt.Errorf("%w: got %q for hostname %q", errInvalidRecordType, hostname.RecordType, hostname.Name))
			}
		}
	}

	return err
}
