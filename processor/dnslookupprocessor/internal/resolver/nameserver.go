// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/cenkalti/backoff/v5"
	"go.uber.org/zap"
)

// NetResolver is a wrapper around net.Resolver to provide custom DNS resolution
// It is used to mock the net.Resolver for testing purposes
type NetResolver struct {
	net.Resolver
}

// Lookup interface defines methods for DNS resolution operations
// It is used to mock the net.Resolver for testing purposes
type Lookup interface {
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
	LookupAddr(ctx context.Context, addr string) ([]string, error)
}

func NewNetResolver(nameserver string, timeout time.Duration) *NetResolver {
	return &NetResolver{
		Resolver: net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
				d := net.Dialer{Timeout: timeout}
				return d.DialContext(ctx, "udp", nameserver)
			},
		},
	}
}

func NewSystemNetResolver(timeout time.Duration) *NetResolver {
	return &NetResolver{
		Resolver: net.Resolver{
			PreferGo: false,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: timeout}
				return d.DialContext(ctx, network, address)
			},
		},
	}
}

func (nr *NetResolver) LookupIP(ctx context.Context, network, host string) ([]net.IP, error) {
	return nr.Resolver.LookupIP(ctx, network, host)
}

func (nr *NetResolver) LookupAddr(ctx context.Context, addr string) ([]string, error) {
	return nr.Resolver.LookupAddr(ctx, addr)
}

// NameserverResolver uses specified DNS servers for resolution
type NameserverResolver struct {
	nameservers []string
	name        string
	maxRetries  int
	resolvers   []Lookup
	timeout     time.Duration
	logger      *zap.Logger
}

// NewNameserverResolver creates a new NameserverResolver with the provided nameservers
func NewNameserverResolver(nameservers []string, timeout time.Duration, maxRetries int, logger *zap.Logger) (*NameserverResolver, error) {
	if len(nameservers) == 0 {
		return nil, errors.New("at least one nameserver must be provided")
	}

	normalizeNameservers, err := validateAndFormatNameservers(nameservers)
	if err != nil {
		return nil, err
	}

	resolvers := make([]Lookup, len(normalizeNameservers))
	for i, ns := range normalizeNameservers {
		nameserver := ns // copy to local
		resolvers[i] = NewNetResolver(nameserver, timeout)
	}

	r := &NameserverResolver{
		name:        "nameservers",
		nameservers: normalizeNameservers,
		maxRetries:  maxRetries,
		resolvers:   resolvers,
		timeout:     timeout,
		logger:      logger,
	}

	return r, nil
}

func NewSystemResolver(timeout time.Duration, maxRetries int, logger *zap.Logger) *NameserverResolver {
	return &NameserverResolver{
		name:        "system",
		nameservers: []string{"system resolver"},
		maxRetries:  maxRetries,
		resolvers:   []Lookup{NewSystemNetResolver(timeout)},
		timeout:     timeout,
		logger:      logger,
	}
}

// Resolve performs a forward DNS lookup (hostname to IP) using the configured nameservers.
// It returns the first IPv4 address found.
// If no IPv4 address is found, it returns the first IP address found.
func (r *NameserverResolver) Resolve(ctx context.Context, hostname string) (string, error) {
	return r.lookupWithNameservers(ctx, hostname, LogKeyHostname, func(resolver Lookup, fnCtx context.Context) (string, error) {
		ips, err := resolver.LookupIP(fnCtx, "ip", hostname)
		if err != nil {
			return "", err
		}
		if len(ips) == 0 {
			return "", ErrNoResolution
		}

		// Find first IPv4 address if available
		for _, ip := range ips {
			if ipv4 := ip.To4(); ipv4 != nil {
				return ipv4.String(), nil
			}
		}

		// If no IPv4, return the first IP
		return ips[0].String(), nil
	})
}

// Reverse performs a reverse DNS lookup (IP to hostname) using the configured nameservers.
func (r *NameserverResolver) Reverse(ctx context.Context, ip string) (string, error) {
	return r.lookupWithNameservers(ctx, ip, LogKeyIP, func(resolver Lookup, fnCtx context.Context) (string, error) {
		hostnames, err := resolver.LookupAddr(fnCtx, ip)
		if err != nil && hostnames == nil {
			return "", err
		}

		// hostname(s) was found but be filtered because of malformed DNS records
		if len(hostnames) == 0 {
			return "", ErrNoResolution
		}

		return NormalizeHostname(hostnames[0]), nil
	})
}

func (r *NameserverResolver) Name() string {
	return r.name
}

func (r *NameserverResolver) Close() error {
	r.resolvers = nil
	return nil
}

func NewExponentialBackOff() *backoff.ExponentialBackOff {
	expBackOff := backoff.ExponentialBackOff{
		InitialInterval:     50 * time.Millisecond,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
		MaxInterval:         200 * time.Millisecond,
	}
	expBackOff.Reset()
	return &expBackOff
}

// lookupWithNameservers attempts a DNS lookup using all configured nameservers.
// The lookupFn is a function that performs the actual DNS lookup using the resolver.
// It retries the lookup with exponential backoff on failure.
// Timeout and temporary errors are retryable failures.
// No resolution is a valid result that no need to retry.
// If all nameservers fail, the last error is returned.
func (r *NameserverResolver) lookupWithNameservers(
	ctx context.Context,
	target string,
	_ string,
	lookupFn func(resolver Lookup, fnCtx context.Context) (string, error),
) (string, error) {
	var lastErr error

	for i, resolver := range r.resolvers {
		expBackOff := NewExponentialBackOff()

		for attempt := 0; attempt <= r.maxRetries; attempt++ {
			result, err := func() (string, error) {
				lookupCtx, cancel := context.WithTimeout(ctx, r.timeout)
				defer cancel()
				return lookupFn(resolver, lookupCtx)
			}()

			// Successful resolution
			if err == nil {
				return result, nil
			}

			// No resolution is a valid result
			if errors.Is(err, ErrNoResolution) {
				return "", err
			}

			lastErr = err

			var e *net.DNSError
			if errors.As(err, &e) {
				// The hostname was not found (NXDOMAIN), we can skip retrying
				if e.IsNotFound {
					return "", ErrNoResolution
				}

				// If the error is not a retryable error, try the next nameserver
				if !e.IsNotFound && !e.IsTimeout && !e.IsTemporary {
					r.logger.Debug("Non retryable DNS error",
						zap.String("lookup", target),
						zap.String("nameserver", r.nameservers[i]),
						zap.Error(err))
					break
				}
			}

			// Sleep for retry
			sleepDuration := expBackOff.NextBackOff()
			if attempt < r.maxRetries {
				select {
				case <-time.After(sleepDuration):
					r.logger.Debug("DNS lookup failed with nameserver. Will retry after backoff.",
						zap.String("lookup", target),
						zap.String("nameserver", r.nameservers[i]),
						zap.String("sleep", sleepDuration.String()),
						zap.Error(err))
				case <-ctx.Done():
					r.logger.Warn("Context cancelled during lookup retry",
						zap.String("lookup", target),
						zap.Error(ctx.Err()))
					return "", ctx.Err()
				}
			}
		}
	}
	return "", lastErr
}

// validateAndFormatNameservers ensures all nameserver addresses have ports and are valid.
// It appends the default DNS port (53) to addresses if it is missing and validates that each
// address contains either a valid IP address or hostname.
// Returns formatted addresses or an error if any address is invalid.
func validateAndFormatNameservers(nameservers []string) ([]string, error) {
	normalizedNameservers := make([]string, len(nameservers))
	for i, ns := range nameservers {
		nns := ns

		// Check if the address has a port; if not, append the default DNS port
		if _, _, err := net.SplitHostPort(ns); err != nil {
			nns = net.JoinHostPort(ns, "53")
		}

		// validate the address
		host, port, _ := net.SplitHostPort(nns)
		_, ipErr := ParseIP(host)
		_, hostErr := ParseHostname(host)
		isPort := govalidator.IsPort(port)
		if !isPort || (ipErr != nil && hostErr != nil) {
			return nil, fmt.Errorf("invalid nameserver address: %s", ns)
		}

		normalizedNameservers[i] = nns
	}
	return normalizedNameservers, nil
}
