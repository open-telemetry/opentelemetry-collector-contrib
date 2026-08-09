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

// defaultServiceInstanceID is the service.instance.id used when the target
// host:port cannot be resolved from the configuration. It has the same
// host:port shape as a resolved id so downstream consumers can parse it
// uniformly.
const defaultServiceInstanceID = "unknown:1433"

// isLocalhost checks if the given host is a local address
func isLocalhost(host string) bool {
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}

// resolveConfiguredHostPort resolves the target host and port from the
// configuration. It is the single source of truth for host/port resolution in
// this receiver, used to dial the endpoint (reachability probe), to identify the
// target (service.instance.id), and to stamp the host.name / server.address /
// server.port resource attributes.
//
// Source priority: DataSource takes precedence over the discrete Server/Port
// fields, which take precedence over ComputerName (Windows performance-counter
// mode). The default SQL Server port (1433) is applied when no port is resolved.
//
// It performs no localhost rewriting and returns the host exactly as configured.
// Callers that want the collector's identity (service.instance.id) layer the
// localhost->os.Hostname rewrite on top via computeServiceInstanceID.
func resolveConfiguredHostPort(cfg *Config) (host string, port int, err error) {
	switch {
	case cfg.DataSource != "":
		// parseDataSource already applies the default port.
		return parseDataSource(cfg.DataSource)
	case cfg.Server != "":
		port = int(cfg.Port)
		if port == 0 {
			port = defaultSQLServerPort
		}
		return cfg.Server, port, nil
	case cfg.ComputerName != "":
		// Windows Performance Counter mode with remote computer: use ComputerName as host.
		return cfg.ComputerName, defaultSQLServerPort, nil
	default:
		// No server specified: no dial target. computeServiceInstanceID rewrites
		// the empty host to os.Hostname for identity purposes.
		return "", defaultSQLServerPort, nil
	}
}

// resolveResourceHostPort resolves the host and port to report as identifying
// resource attributes: host.name, server.address, server.port, and (via
// computeServiceInstanceID) service.instance.id.
//
// It layers a localhost/empty -> os.Hostname rewrite on top of
// resolveConfiguredHostPort so that every attribute the receiver reports as the
// target's identity agrees on the same hostname. A resource attribute of
// "localhost" is ambiguous across hosts, whereas the reachability probe's dial
// target must stay raw (a loopback address is exactly what should be dialed for
// a locally-configured server) -- that path calls resolveConfiguredHostPort
// directly and must not use this function.
func resolveResourceHostPort(cfg *Config) (host string, port int, err error) {
	host, port, err = resolveConfiguredHostPort(cfg)
	if err != nil {
		return host, port, err
	}

	if isLocalhost(host) || host == "" {
		hostname, hostErr := os.Hostname()
		if hostErr != nil {
			return host, port, hostErr
		}
		host = hostname
	}

	return host, port, nil
}

// computeServiceInstanceID computes the service.instance.id based on the configuration
// Format: <host>:<port>
// Special handling:
// - localhost/127.0.0.1 are replaced with os.Hostname()
// - Port 0 defaults to 1433
func computeServiceInstanceID(cfg *Config) (string, error) {
	host, port, err := resolveResourceHostPort(cfg)
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
