// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/systemscraper"

import (
	"strings"

	"go.uber.org/zap"
)

// BasicInterface represents a basic interface structure for interface counting
type BasicInterface struct {
	Name string
}

// parseInterfaceOutput parses Cisco interface command output and extracts interface names
func parseInterfaceOutput(output string, logger *zap.Logger) []*BasicInterface {
	logger.Debug("Parsing interface output", zap.Int("output_length", len(output)))

	var interfaces []*BasicInterface
	lines := strings.Split(output, "\n")

	// Parse interface definitions from command output
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for common Cisco interface name patterns
		if strings.Contains(line, "Ethernet") || strings.Contains(line, "Loopback") || strings.Contains(line, "Vlan") {
			// Extract interface name from line
			parts := strings.Fields(line)
			if len(parts) > 0 {
				intfName := parts[0]
				if strings.Contains(intfName, "Ethernet") || strings.Contains(intfName, "Loopback") || strings.Contains(intfName, "Vlan") {
					interfaces = append(interfaces, &BasicInterface{
						Name: intfName,
					})
					logger.Debug("Found interface", zap.String("name", intfName))
				}
			}
		}
	}

	return interfaces
}
