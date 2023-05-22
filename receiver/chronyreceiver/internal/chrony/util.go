// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package chrony // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/chrony"

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"go.uber.org/multierr"
)

var (
	ErrInvalidNetwork = errors.New("invalid network format")
)

// SplitNetworkEndpoint takes in a URL like string of the format: [network type]://[network endpoint]
// and then will return the network and the endpoint for the client to use for connection.
func SplitNetworkEndpoint(addr string) (network, endpoint string, err error) {
	parts := strings.SplitN(addr, "://", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("address %s missing '://' to separate networks: %w", addr, ErrInvalidNetwork)
	}

	network, endpoint = parts[0], parts[1]
	switch network {
	case "udp":
		host, _, err := net.SplitHostPort(endpoint)
		if err != nil {
			return "", "", fmt.Errorf("issue parsing endpoint: %w", multierr.Combine(ErrInvalidNetwork, err))
		}
		if host == "" {
			return "", "", fmt.Errorf("missing hostname: %w", ErrInvalidNetwork)
		}
	case "unix", "unixgram":
		if _, err := os.Stat(endpoint); err != nil {
			return "", "", err
		}
		// Chrony uses socket type DGRAM which converts to `unixgram`,
		// in order to preserve configuration of existing clients, this will overwrite the network type
		network = "unixgram"
	default:
		return "", "", fmt.Errorf("unknown network %s: %w", network, ErrInvalidNetwork)
	}

	return network, endpoint, nil
}
