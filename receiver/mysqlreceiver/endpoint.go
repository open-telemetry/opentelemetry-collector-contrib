// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"net"
	"os"
	"strconv"

	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/zap"
)

// resolvedEndpoint is the network location of the monitored MySQL instance, derived once from the
// configured endpoint. It carries both the server.address / server.port resource attribute values
// and the seed hashed into service.instance.id, so the attributes cannot name different machines.
type resolvedEndpoint struct {
	// address is the server.address value. It is empty when the endpoint could not be parsed, in
	// which case neither server.address nor server.port is reported.
	address string
	port    int64
	// hasPort reports whether port holds a usable value. A Unix socket has no port.
	hasPort bool
	// instanceIDSeed is the string hashed into the service.instance.id UUID.
	instanceIDSeed string
}

// isLoopbackHost reports whether host names the machine the collector runs on.
func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// resolveServerEndpoint resolves the configured endpoint into the values reported as
// server.address, server.port and the service.instance.id seed.
//
// A loopback host (localhost, 127.0.0.1, ::1) is replaced with the host name of the machine running
// the collector. Loopback is only reachable when the instance is co-located with the collector, so
// the collector host's name identifies the instance better than "localhost", which every monitored
// host would report identically. See
// https://github.com/open-telemetry/semantic-conventions/issues/4026.
//
// The endpoint is immutable configuration, so this is called once at scraper construction rather
// than per resource: it avoids an os.Hostname call for every emitted resource, and it means
// server.address and service.instance.id name the same machine as of the same moment instead of one
// being frozen at startup while the other tracks later changes.
func resolveServerEndpoint(cfg *Config, logger *zap.Logger) resolvedEndpoint {
	endpoint := cfg.AddrConfig.Endpoint

	// With a Unix socket transport the endpoint is the socket path, which semantic conventions
	// report verbatim as server.address. There is no port.
	if cfg.AddrConfig.Transport == confignet.TransportTypeUnix {
		return resolvedEndpoint{address: endpoint, instanceIDSeed: endpoint}
	}

	host, portString, err := net.SplitHostPort(endpoint)
	if err != nil {
		logger.Warn("Failed to parse endpoint; server.address and server.port will not be reported and the raw endpoint is used as the service.instance.id UUID seed",
			zap.String("endpoint", endpoint),
			zap.Error(err))
		return resolvedEndpoint{instanceIDSeed: endpoint}
	}

	// The seed keeps the raw endpoint unless a loopback host is rewritten, which leaves every
	// service.instance.id emitted before this resolution existed unchanged.
	resolved := resolvedEndpoint{address: host, instanceIDSeed: endpoint}

	if isLoopbackHost(host) {
		hostname, hostnameErr := os.Hostname()
		if hostnameErr != nil {
			logger.Warn("Failed to resolve the collector host name for a loopback endpoint; server.address and service.instance.id may not be unique across machines",
				zap.String("endpoint", endpoint),
				zap.Error(hostnameErr))
		} else {
			resolved.address = hostname
			resolved.instanceIDSeed = net.JoinHostPort(hostname, portString)
		}
	}

	port, err := strconv.ParseInt(portString, 10, 64)
	if err != nil {
		logger.Warn("Failed to parse endpoint port; server.address and server.port will not be reported",
			zap.String("endpoint", endpoint),
			zap.Error(err))
		resolved.address = ""
		return resolved
	}

	resolved.port = port
	resolved.hasPort = true
	return resolved
}
