// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/resolver"

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
)

// ErrInvalidHostFilePath Error for invalid host file path
var ErrInvalidHostFilePath = errors.New("host file does not exist")

// HostFileResolver uses host files for DNS resolution
type HostFileResolver struct {
	hostnameToIP map[string][]string // For forward lookups
	ipToHostname map[string][]string // For reverse lookups
	logger       *zap.Logger
}

// NewHostFileResolver creates a new HostFileResolver with the provided host file paths.
// The entries of later hostfiles are appended to the mapping.
func NewHostFileResolver(hostFilePaths []string, logger *zap.Logger) (*HostFileResolver, error) {
	if len(hostFilePaths) == 0 {
		return nil, ErrInvalidHostFilePath
	}

	r := &HostFileResolver{
		ipToHostname: make(map[string][]string),
		hostnameToIP: make(map[string][]string),
		logger:       logger,
	}

	// Load all host files
	for _, path := range hostFilePaths {
		if err := r.parseHostFile(path); err != nil {
			return nil, err
		}
	}
	deduplicateMapping(r.ipToHostname)
	deduplicateMapping(r.hostnameToIP)

	return r, nil
}

// parseHostFile parses a host file and builds resolver maps
func (r *HostFileResolver) parseHostFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrInvalidHostFilePath
		}
		return fmt.Errorf(`failed to open host file "%s": %w`, path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines and comments
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Remove anything after #
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}

		// Split the line into fields
		fields := strings.Fields(line)

		// Skip lines with only IP and no hostnames
		if len(fields) < 2 {
			continue
		}

		// First field is IP
		ip := fields[0]

		// Validate IP
		if _, err := ValidateIP(ip); err != nil {
			r.logger.Debug("Invalid IP address in host file",
				zap.String("path", path),
				zap.Int("line", lineNum),
				zap.String("ip", ip))
			continue
		}

		// Process all hostnames for this IP
		for _, hostname := range fields[1:] {
			normalizedHostname, err := ValidateHostname(NormalizeHostname(hostname))
			if err != nil {
				r.logger.Debug("Invalid hostname in host file",
					zap.String("path", path),
					zap.Int("line", lineNum),
					zap.String("hostname", normalizedHostname))
				continue
			}
			r.hostnameToIP[normalizedHostname] = append(r.hostnameToIP[normalizedHostname], ip)
			r.ipToHostname[ip] = append(r.ipToHostname[ip], normalizedHostname)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

// Resolve performs a forward DNS lookup (hostname to IP) using the loaded host files
func (r *HostFileResolver) Resolve(_ context.Context, hostname string) ([]string, error) {
	if ips, found := r.hostnameToIP[hostname]; found {
		return ips, nil
	}

	return nil, nil
}

// Reverse performs a reverse DNS lookup (IP to hostname) using the loaded host files
func (r *HostFileResolver) Reverse(_ context.Context, ip string) ([]string, error) {
	if hostnames, found := r.ipToHostname[ip]; found {
		return hostnames, nil
	}

	return nil, nil
}

func deduplicateMapping(mapping map[string][]string) {
	for key, vals := range mapping {
		mapping[key] = deduplicateStrings(vals)
	}
}

// deduplicateStrings removes duplicate strings from a slice while preserving order.
func deduplicateStrings(input []string) []string {
	seen := make(map[string]struct{})
	var result []string

	for _, val := range input {
		if _, exists := seen[val]; !exists {
			seen[val] = struct{}{}
			result = append(result, val)
		}
	}
	return result
}
