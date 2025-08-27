// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const defaultSQLServerPort = 1433

// computeServiceInstanceID computes the service.instance.id based on the configuration
// Format: <host>:<port>
// Special handling:
// - localhost/127.0.0.1 are replaced with os.Hostname()
// - Port 0 defaults to 1433
// - DataSource takes precedence over Server/Port config
func computeServiceInstanceID(cfg *Config) (string, error) {
	var host string
	var port int

	// Priority 1: Parse from DataSource if present
	if cfg.DataSource != "" {
		h, p, err := parseDataSource(cfg.DataSource)
		if err != nil {
			return "", fmt.Errorf("failed to parse datasource: %w", err)
		}
		host = h
		port = p
	} else if cfg.Server != "" {
		// Priority 2: Use explicit server and port
		host = cfg.Server
		port = int(cfg.Port)
	} else {
		// Priority 3: No server specified, use os.Hostname()
		hostname, err := os.Hostname()
		if err != nil {
			return "", fmt.Errorf("failed to get hostname: %w", err)
		}
		host = hostname
		port = defaultSQLServerPort
	}

	// Handle localhost special case
	if host == "localhost" || host == "127.0.0.1" {
		hostname, err := os.Hostname()
		if err != nil {
			return "", fmt.Errorf("failed to get hostname for localhost replacement: %w", err)
		}
		host = hostname
	}

	// Ensure default port when not specified
	if port == 0 {
		port = defaultSQLServerPort
	}

	return fmt.Sprintf("%s:%d", host, port), nil
}

// parseDataSource extracts server and port from SQL Server connection string
// Supports various formats:
// - server=myserver,1433;...
// - server=myserver:1433;...
// - server=myserver;port=1433;...
// - Data Source=myserver,1433;...
// - server=myserver\instance,1433;... (named instances)
func parseDataSource(dataSource string) (string, int, error) {
	if dataSource == "" {
		return "", 0, fmt.Errorf("datasource is empty")
	}

	var server string
	port := defaultSQLServerPort

	// Normalize spaces around separators for consistent parsing
	normalizedDS := strings.ReplaceAll(dataSource, " =", "=")
	normalizedDS = strings.ReplaceAll(normalizedDS, "= ", "=")
	normalizedDS = strings.ReplaceAll(normalizedDS, " ,", ",")
	normalizedDS = strings.ReplaceAll(normalizedDS, ", ", ",")
	normalizedDS = strings.ReplaceAll(normalizedDS, " :", ":")
	normalizedDS = strings.ReplaceAll(normalizedDS, ": ", ":")
	normalizedDS = strings.ReplaceAll(normalizedDS, " ;", ";")
	normalizedDS = strings.ReplaceAll(normalizedDS, "; ", ";")

	// Create lowercase version for case-insensitive keyword matching
	lowerDS := strings.ToLower(normalizedDS)

	// Try to extract server parameter using regex
	// Matches: server=<value> or data source=<value>
	// Value can optionally be followed by :<port> or ,<port>
	serverPattern := regexp.MustCompile(`(?i)(?:server|data\s+source)=([^;,:\s]+(?:\\[^;,:\s]+)?)(?:[,:](\d+))?`)
	matches := serverPattern.FindStringSubmatch(normalizedDS)

	if len(matches) > 1 && matches[1] != "" {
		server = matches[1]

		// Check if port is included with server (matches[2])
		if len(matches) > 2 && matches[2] != "" {
			if p, err := strconv.Atoi(matches[2]); err == nil {
				port = p
			}
		}
	}

	// Check for separate port parameter
	// This takes precedence over port extracted from server string
	portPattern := regexp.MustCompile(`(?i)port=(\d+)`)
	if portMatches := portPattern.FindStringSubmatch(lowerDS); len(portMatches) > 1 {
		if p, err := strconv.Atoi(portMatches[1]); err == nil {
			port = p
		}
	}

	if server == "" {
		return "", 0, fmt.Errorf("could not extract server from datasource")
	}

	return server, port, nil
}