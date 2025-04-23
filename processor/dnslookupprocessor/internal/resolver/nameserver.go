// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/cenkalti/backoff/v5"
	"go.uber.org/zap"
)

// NameserverResolver uses specified DNS servers for resolution
type NameserverResolver struct {
	nameservers []string
	name        string
	maxRetries  int
	resolvers   []*net.Resolver
	timeout     time.Duration
	logger      *zap.Logger
}

// NewNameserverResolver creates a new NameserverResolver with the provided nameservers
func NewNameserverResolver(nameservers []string, timeout time.Duration, maxRetries int, logger *zap.Logger) (*NameserverResolver, error) {
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
		resolvers:   []*net.Resolver{net.DefaultResolver},
		timeout:     timeout,
		logger:      logger,
	}
}

// Resolve performs a forward DNS lookup (hostname to IP) using the configured nameservers.
func (r *NameserverResolver) Resolve(ctx context.Context, hostname string) (string, error) {
	return r.lookupWithNameservers(ctx, hostname, LogKeyHostname, func(resolver *net.Resolver, fnCtx context.Context) (string, error) {
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
	return r.lookupWithNameservers(ctx, ip, LogKeyIP, func(resolver *net.Resolver, fnCtx context.Context) (string, error) {
		hostnames, err := resolver.LookupAddr(fnCtx, ip)
		if err != nil && hostnames == nil {
			return "", err
		}

		// hostname(s) was found but be filtered because of malformed DNS records
		if len(hostnames) == 0 {
			return "", ErrNoResolution
		}

		return removeTrailingDot(hostnames[0]), nil
	})
}

func (r *NameserverResolver) Name() string {
	return r.name
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
// It returns the first successful result or the last error encountered.
func (r *NameserverResolver) lookupWithNameservers(
	ctx context.Context,
	target string,
	logKey string,
	lookupFn func(resolver *net.Resolver, fnCtx context.Context) (string, error),
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

			sleepDuration := expBackOff.NextBackOff()
			r.logger.Debug("DNS lookup failed with nameserver. Will retry after backoff.",
				zap.String("lookup", target),
				zap.String("nameserver", r.nameservers[i]),
				zap.String("sleep", sleepDuration.String()),
				zap.Error(err))

			// Sleep for retry
			if attempt < r.maxRetries {
				select {
				case <-time.After(sleepDuration):
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
