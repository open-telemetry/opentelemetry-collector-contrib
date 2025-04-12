// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"go.uber.org/zap"
)

// NameserverResolver uses specified DNS servers for resolution
type NameserverResolver struct {
	nameservers []string
	resolvers   []*net.Resolver
	timeout     time.Duration
	logger      *zap.Logger
}

// NewNameserverResolver creates a new NameserverResolver with the provided nameservers
func NewNameserverResolver(nameservers []string, timeout time.Duration, logger *zap.Logger) (*NameserverResolver, error) {
	if len(nameservers) == 0 {
		return nil, errors.New("at least one nameserver must be provided")
	}

	normalizeNameservers, err := normalizeNameserverAddresses(nameservers)
	if err != nil {
		return nil, err
	}

	resolvers := make([]*net.Resolver, len(normalizeNameservers))
	for i, ns := range normalizeNameservers {
		ns := ns // copy to local
		resolvers[i] = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: timeout}
				return d.DialContext(ctx, "udp", ns)
			},
		}
	}

	r := &NameserverResolver{
		nameservers: normalizeNameservers,
		resolvers:   resolvers,
		timeout:     timeout,
		logger:      logger,
	}

	return r, nil
}

func NewSystemResolver(timeout time.Duration, logger *zap.Logger) *NameserverResolver {
	return &NameserverResolver{
		nameservers: []string{"system_resolver"},
		resolvers:   []*net.Resolver{net.DefaultResolver},
		timeout:     timeout,
		logger:      logger,
	}
}

// Resolve performs a forward DNS lookup (hostname to IP) using the configured nameservers.
func (r *NameserverResolver) Resolve(ctx context.Context, hostname string) (string, error) {
	return r.lookupWithNameservers(ctx, hostname, func(resolver *net.Resolver, fnCtx context.Context) (string, error) {
		ips, err := resolver.LookupIP(fnCtx, "ip", hostname)
		if err != nil {
			return "", err
		}
		if len(ips) == 0 {
			return "", ErrNoResolution
		}
		return ips[0].String(), nil
	})
}

// Reverse performs a reverse DNS lookup (IP to hostname) using the configured nameservers.
func (r *NameserverResolver) Reverse(ctx context.Context, ip string) (string, error) {
	return r.lookupWithNameservers(ctx, ip, func(resolver *net.Resolver, fnCtx context.Context) (string, error) {
		hostnames, err := resolver.LookupAddr(fnCtx, ip)
		if err != nil {
			return "", err
		}
		if len(hostnames) == 0 {
			return "", ErrNoResolution
		}
		return standardizeHostname(hostnames[0]), nil
	})
}

// lookupWithNameservers attempts a DNS lookup using all configured nameservers.
// It returns the first successful result or the last error encountered.
func (r *NameserverResolver) lookupWithNameservers(
	ctx context.Context,
	query string,
	lookupFn func(resolver *net.Resolver, fnCtx context.Context) (string, error),
) (string, error) {
	lookupCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	lastErr := ErrNoResolution

	for i, resolver := range r.resolvers {
		result, err := lookupFn(resolver, lookupCtx)
		if err == nil {
			// Successful resolution
			return result, nil
		} else {
			r.logger.Debug("DNS lookup failed with nameserver",
				zap.String("query", query),
				zap.String("nameserver", r.nameservers[i]),
				zap.Error(err))
			lastErr = err
		}
	}

	return "", lastErr
}

// normalizeNameserverAddresses normalizes the nameserver addresses by ensuring they have a port
func normalizeNameserverAddresses(nameservers []string) ([]string, error) {
	normalizedNameservers := make([]string, len(nameservers))
	for i, ns := range nameservers {
		// Check if the address has a port; if not, append the default DNS port
		if _, _, err := net.SplitHostPort(ns); err != nil {
			nns := net.JoinHostPort(ns, "53")

			// Re-validate the address
			if _, _, err := net.SplitHostPort(nns); err != nil {
				return nil, fmt.Errorf("invalid nameserver address: %s", ns)
			}

			normalizedNameservers[i] = nns
		} else {
			normalizedNameservers[i] = ns
		}
	}
	return normalizedNameservers, nil
}

// removeTrailingDot removes a trailing dot from a hostname if present
// Note: LookupAddr results typically have a trailing dot which can be removed
func removeTrailingDot(hostname string) string {
	if len(hostname) > 0 && hostname[len(hostname)-1] == '.' {
		return hostname[:len(hostname)-1]
	}
	return hostname
}

// standardizeHostname removes trailing dot and converts hostname to lowercase
func standardizeHostname(hostname string) string {
	// Remove trailing dot
	hostname = removeTrailingDot(hostname)

	// Convert to lowercase
	hostname = strings.ToLower(hostname)

	return hostname
}
