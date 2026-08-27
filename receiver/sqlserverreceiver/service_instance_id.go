// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/microsoft/go-mssqldb/msdsn"
)

const defaultSQLServerPort = 1433

// isLocalhost checks if the given host is a local address
func isLocalhost(host string) bool {
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}

// resolveServerEndpoint determines the network location of the monitored SQL Server
// instance, reported as the server.address and server.port resource attributes.
// Special handling:
// - localhost/127.0.0.1/::1 are replaced with os.Hostname()
// - Port 0 defaults to 1433
//
// A loopback target is only reachable when the server is co-located with the collector,
// so the collector host's name is a more useful server identity than "localhost", which
// would be shared by every monitored host.
// See https://github.com/open-telemetry/semantic-conventions/issues/4026.
func resolveServerEndpoint(cfg *Config) (string, int, error) {
	var host string
	var port int

	// Parse connection details based on configuration priority
	switch {
	case cfg.DataSource != "":
		h, p, err := parseDataSource(cfg.DataSource)
		if err != nil {
			return "", 0, fmt.Errorf("failed to parse datasource: %w", err)
		}
		host, port = h, p
	case cfg.Server != "":
		host, port = cfg.Server, int(cfg.Port)
	case cfg.ComputerName != "":
		// Windows Performance Counter mode with remote computer: use ComputerName as host
		host, port = cfg.ComputerName, defaultSQLServerPort
	default:
		// No server specified, use hostname with default port
		hostname, err := os.Hostname()
		if err != nil {
			return "", 0, err
		}
		host, port = hostname, defaultSQLServerPort
	}

	// Replace localhost with actual hostname
	if isLocalhost(host) || host == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return "", 0, err
		}
		host = hostname
	}

	// Apply default port if not specified
	if port == 0 {
		port = defaultSQLServerPort
	}

	return host, port, nil
}

// computeServiceInstanceID computes the service.instance.id based on the configuration
// Format: <host>:<port>, using the same endpoint resolution as server.address/server.port.
func computeServiceInstanceID(cfg *Config) (string, error) {
	host, port, err := resolveServerEndpoint(cfg)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%d", host, port), nil
}

// parseDataSource extracts server and port from SQL Server connection string
// Uses the microsoft/go-mssqldb library's built-in parser for accurate parsing
func parseDataSource(dataSource string) (string, int, error) {
	if dataSource == "" {
		return "", 0, errors.New("datasource is empty")
	}

	// Parse the connection string using the go-mssqldb library
	config, err := msdsn.Parse(dataSource)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse datasource: %w", err)
	}

	// Apply default port if not specified
	port := int(config.Port)
	if port == 0 {
		port = defaultSQLServerPort
	}

	return config.Host, port, nil
}
