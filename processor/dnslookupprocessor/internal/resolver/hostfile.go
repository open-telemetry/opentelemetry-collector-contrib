// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"bufio"
	"context"
	"errors"
	"os"
	"strings"

	"go.uber.org/zap"
)

// ErrInvalidHostFilePath Error for invalid host file path
var ErrInvalidHostFilePath = errors.New("invalid host file path")

// HostFileResolver uses host files for DNS resolution
type HostFileResolver struct {
	name         string
	hostnameToIP map[string]string // For forward lookups
	ipToHostname map[string]string // For reverse lookups
	logger       *zap.Logger
}

// NewHostFileResolver creates a new HostFileResolver with the provided host file paths
func NewHostFileResolver(hostFilePaths []string, logger *zap.Logger) (*HostFileResolver, error) {
	if len(hostFilePaths) == 0 {
		return nil, ErrInvalidHostFilePath
	}

	r := &HostFileResolver{
		name:         "hostfiles",
		ipToHostname: make(map[string]string),
		hostnameToIP: make(map[string]string),
		logger:       logger,
	}

	// Load all host files
	for _, path := range hostFilePaths {
		if err := r.parseHostFile(path); err != nil {
			return nil, err
		}
	}

	r.logger.Info("Number of records in hostfiles",
		zap.Int("IPs", len(r.ipToHostname)),
		zap.Int("Hostnames", len(r.hostnameToIP)))

	return r, nil
}

// parseHostFile parses a host file and builds resolver maps
func (r *HostFileResolver) parseHostFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		r.logger.Error("Failed to open host file",
			zap.String("path", path),
			zap.Error(err))
		return err
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
		if len(fields) < 2 {
			r.logger.Debug("Skipping invalid host file entry",
				zap.String("path", path),
				zap.Int("line", lineNum),
				zap.String("content", line))
			continue
		}

		// First field is IP
		ip := fields[0]

		// Validate IP
		if _, err := ParseIP(ip); err != nil {
			r.logger.Debug("Invalid IP address in host file",
				zap.String("path", path),
				zap.Int("line", lineNum),
				zap.String("ip", ip))
			continue
		}

		// Process all hostnames for this IP
		for _, hostname := range fields[1:] {
			hostname, err := ParseHostname(strings.ToLower(hostname))
			if err != nil {
				r.logger.Debug("Invalid hostname in host file",
					zap.String("path", path),
					zap.Int("line", lineNum),
					zap.String("hostname", hostname))
				continue
			}
			r.hostnameToIP[hostname] = ip
			r.ipToHostname[ip] = hostname
		}
	}

	if err := scanner.Err(); err != nil {
		r.logger.Error("Error reading host file",
			zap.String("path", path),
			zap.Error(err))
		return err
	}

	return nil
}

// Resolve performs a forward DNS lookup (hostname to IP) using the loaded host files
func (r *HostFileResolver) Resolve(ctx context.Context, hostname string) (string, error) {
	if ip, found := r.hostnameToIP[hostname]; found {
		return ip, nil
	}

	return "", ErrNotInHostFiles
}

// Reverse performs a reverse DNS lookup (IP to hostname) using the loaded host files
func (r *HostFileResolver) Reverse(ctx context.Context, ip string) (string, error) {
	if hostname, found := r.ipToHostname[ip]; found {
		return hostname, nil
	}

	return "", ErrNotInHostFiles
}

func (r *HostFileResolver) Name() string {
	return r.name
}
