// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"

import "strings"

// TCPCheckErrorCode represents the type of error that occurred during a TCP check
type TCPCheckErrorCode int

const (
	// ConnectionRefused indicates the target actively refused the connection
	ConnectionRefused TCPCheckErrorCode = iota
	// ConnectionTimeout indicates the connection attempt timed out
	ConnectionTimeout
	// InvalidEndpoint indicates the endpoint format is invalid
	InvalidEndpoint
	// NetworkUnreachable indicates the network is unreachable
	NetworkUnreachable
	// UnknownError indicates an unknown error occurred
	UnknownError
)

// String returns the string representation of the error code
func (c TCPCheckErrorCode) String() string {
	switch c {
	case ConnectionRefused:
		return "connection_refused"
	case ConnectionTimeout:
		return "connection_timeout"
	case InvalidEndpoint:
		return "invalid_endpoint"
	case NetworkUnreachable:
		return "network_unreachable"
	case UnknownError:
		return "unknown_error"
	default:
		return "unknown_error"
	}
}

// GetErrorCode converts a raw error message to a standardized error code
func GetErrorCode(err error) TCPCheckErrorCode {
	if err == nil {
		return UnknownError
	}

	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "connection refused"):
		return ConnectionRefused
	case strings.Contains(errStr, "timeout"):
		return ConnectionTimeout
	case strings.Contains(errStr, "invalid endpoint"):
		return InvalidEndpoint
	case strings.Contains(errStr, "network is unreachable"):
		return NetworkUnreachable
	default:
		return UnknownError
	}
}
