// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"errors"
)

const (
	LogKeyHostname = "hostname"
	LogKeyIP       = "ip"
)

var (
	// ErrNoResolution indicates no resolution was found for the provided hostname or IP
	ErrNoResolution = errors.New("no resolution found")

	// ErrNotInHostFiles indicates no resolution was found in host files
	ErrNotInHostFiles = errors.New("not found in host files")

	// ErrInvalidHostname indicates the provided hostname is invalid
	ErrInvalidHostname = errors.New("invalid hostname format")

	// ErrInvalidIP indicates the provided IP address is invalid
	ErrInvalidIP = errors.New("invalid IP address format")
)

// Resolver defines methods for DNS resolution operations
type Resolver interface {
	// Resolve performs forward DNS resolution (hostname to IP)
	// Returns IP address as string or error if resolution fails
	Resolve(ctx context.Context, hostname string) (string, error)

	// Reverse performs reverse DNS resolution (IP to hostname)
	// Returns hostname as string or error if resolution fails
	Reverse(ctx context.Context, ip string) (string, error)

	Name() string
}
