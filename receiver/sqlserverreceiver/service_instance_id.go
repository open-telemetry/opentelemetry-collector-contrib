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

// resolveEndpoint parses the configured connection details into the host, named instance and port
// that locate the monitored SQL Server instance. The named instance is only ever set when
// connecting via a datasource; the port is returned as configured, so callers that need a concrete
// port must apply defaultSQLServerPort themselves.
//
// A loopback target (localhost, 127.0.0.1, ::1) or an unset host is replaced with the collector
// host's name. Loopback is only reachable when the server is co-located with the collector, so the
// collector host's name identifies the server better than "localhost", which every monitored host
// would share. See https://github.com/open-telemetry/semantic-conventions/issues/4026.
func resolveEndpoint(cfg *Config) (host, instance string, port int, err error) {
	switch {
	case cfg.DataSource != "":
		config, parseErr := parseDataSource(cfg.DataSource)
		if parseErr != nil {
			return "", "", 0, fmt.Errorf("failed to parse datasource: %w", parseErr)
		}
		host, instance, port = config.Host, config.Instance, int(config.Port)
	case cfg.Server != "":
		host, port = cfg.Server, int(cfg.Port)
	case cfg.ComputerName != "":
		// Windows Performance Counter mode with remote computer: use ComputerName as host
		host, port = cfg.ComputerName, defaultSQLServerPort
	default:
		// No server specified, use hostname with default port
		hostname, hostErr := os.Hostname()
		if hostErr != nil {
			return "", "", 0, hostErr
		}
		host, port = hostname, defaultSQLServerPort
	}

	// Replace localhost with actual hostname
	if isLocalhost(host) || host == "" {
		hostname, hostErr := os.Hostname()
		if hostErr != nil {
			return "", "", 0, hostErr
		}
		host = hostname
	}

	return host, instance, port, nil
}

// resolveServerEndpoint determines the network location of the monitored SQL Server instance,
// reported as the server.address and server.port resource attributes. The port defaults to 1433
// when not configured, including for a named instance, whose port is negotiated at connect time.
func resolveServerEndpoint(cfg *Config) (string, int, error) {
	host, _, port, err := resolveEndpoint(cfg)
	if err != nil {
		return "", 0, err
	}

	if port == 0 {
		port = defaultSQLServerPort
	}

	return host, port, nil
}

// computeServiceInstanceID computes the service.instance.id based on the configuration.
// Datasource format precedence: <host>\<instance>, then <host>:<port> (default 1433).
// The host is resolved the same way as server.address.
func computeServiceInstanceID(cfg *Config) (string, error) {
	host, instance, port, err := resolveEndpoint(cfg)
	if err != nil {
		return "", err
	}

	if instance != "" {
		return fmt.Sprintf(`%s\%s`, host, instance), nil
	}

	if port == 0 {
		port = defaultSQLServerPort
	}

	return fmt.Sprintf("%s:%d", host, port), nil
}

// parseDataSource extracts SQL Server connection details without replacing an omitted port.
// Uses the microsoft/go-mssqldb library's built-in parser for accurate parsing.
func parseDataSource(dataSource string) (msdsn.Config, error) {
	if dataSource == "" {
		return msdsn.Config{}, errors.New("datasource is empty")
	}

	// Parse the connection string using the go-mssqldb library
	config, err := msdsn.Parse(dataSource)
	if err != nil {
		return msdsn.Config{}, fmt.Errorf("failed to parse datasource: %w", err)
	}

	return config, nil
}
