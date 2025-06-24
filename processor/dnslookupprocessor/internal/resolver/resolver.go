// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/resolver"

import (
	"context"
	"errors"
)

var (
	// ErrInvalidHostname indicates the provided hostname is invalid
	ErrInvalidHostname = errors.New("invalid hostname format")

	// ErrInvalidIP indicates the provided IP address is invalid
	ErrInvalidIP = errors.New("invalid IP address format")
)

// Resolver defines methods for DNS resolution operations
type Resolver interface {
	// Resolve performs a forward DNS resolution (hostname to IP).
	//
	// It returns:
	//   - ([]string, nil) on successful resolution
	//   - ([]string{}, nil) if the server responded, but returned no resolution (empty slice)
	//   - (nil, nil) if no resolution is found and expected the caller to try the next resolver
	//   - (nil, error) if an error occurred
	Resolve(ctx context.Context, hostname string) ([]string, error)

	// Reverse performs reverse DNS resolution (IP to hostname)
	//
	// It returns:
	//   - ([]string, nil) on successful resolution
	//   - ([]string{}, nil) if the server responded, but returned no resolution (empty slice)
	//   - (nil, nil) if no resolution is found and expected the caller to try the next resolver
	//   - (nil, error) if an error occurred
	Reverse(ctx context.Context, ip string) ([]string, error)
}
