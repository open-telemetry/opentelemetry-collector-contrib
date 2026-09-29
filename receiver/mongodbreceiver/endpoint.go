// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
)

// serverAddressAndPort reports the network location the server gives for itself, which is what the
// server.address and server.port resource attributes describe. Taking it from the server rather
// than from the configured endpoint is what lets each replica set member emit its own resource.
//
// The value is a host with an optional port. An IPv6 literal may arrive bare or bracketed, so the
// port is split off with net.SplitHostPort rather than by counting colons; a bare "::1" would
// otherwise be read as a malformed host and abort the scrape before any resource is emitted.
func serverAddressAndPort(serverStatus bson.M) (string, int64, error) {
	host, ok := serverStatus["host"].(string)
	if !ok {
		return "", 0, errors.New("host field not found in server status")
	}

	if address, portString, err := net.SplitHostPort(host); err == nil {
		port, parseErr := strconv.ParseInt(portString, 10, 64)
		if parseErr != nil {
			return "", 0, fmt.Errorf("failed to parse port: %w", parseErr)
		}
		return address, port, nil
	}

	// No port was separable, so the value is either a host on its own or an IPv6 literal whose
	// colons SplitHostPort read as separators.
	address := strings.Trim(host, "[]")
	if strings.Contains(address, ":") && net.ParseIP(address) == nil {
		return "", 0, fmt.Errorf("unexpected host format: %s", host)
	}
	return address, defaultMongoDBPort, nil
}

// isLoopbackHost reports whether host names the machine the collector runs on.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// resolveLoopbackHost returns collectorHostName when host is a loopback address. Loopback is only
// reachable when the instance is co-located with the collector, so the collector host's name
// identifies the instance better than "localhost", which every monitored host would report
// identically. See https://github.com/open-telemetry/semantic-conventions/issues/4026.
//
// A host that is not loopback, and a collector host name that could not be determined, both leave
// the reported address unchanged.
func resolveLoopbackHost(host, collectorHostName string) string {
	if collectorHostName == "" || !isLoopbackHost(host) {
		return host
	}
	return collectorHostName
}

// resolveCollectorHostName reads the host name of the machine running the collector, used in place
// of a loopback server address. It is read once at scraper construction rather than per scrape,
// since every scrape of every monitored node would otherwise repeat the lookup.
func resolveCollectorHostName(logger *zap.Logger) string {
	hostname, err := os.Hostname()
	if err != nil {
		logger.Warn("Failed to resolve the collector host name; a loopback server.address will be reported as configured and may not be unique across machines",
			zap.Error(err))
		return ""
	}
	return hostname
}
