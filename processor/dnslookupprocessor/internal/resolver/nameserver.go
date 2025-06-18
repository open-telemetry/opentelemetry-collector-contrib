// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/resolver"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/cenkalti/backoff/v5"
)

// Lookup interface defines methods for DNS resolution operations
// It is used to mock the net.Resolver
type Lookup interface {
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
	LookupAddr(ctx context.Context, addr string) ([]string, error)
}

func newNetResolver(nameserver string, timeout time.Duration) *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: timeout}
			return d.DialContext(ctx, network, nameserver)
		},
	}
}

func newSystemNetResolver(timeout time.Duration) *net.Resolver {
	return &net.Resolver{
		PreferGo: false,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: timeout}
			return d.DialContext(ctx, network, address)
		},
	}
}

// NameserverResolver uses specified DNS servers for resolution
type NameserverResolver struct {
	maxRetries int
	resolvers  []Lookup
	timeout    time.Duration
}

// NewNameserverResolver creates a new NameserverResolver that uses the provided nameservers for DNS resolution
func NewNameserverResolver(nameservers []string, timeout time.Duration, maxRetries int) (*NameserverResolver, error) {
	if len(nameservers) == 0 {
		return nil, errors.New("at least one nameserver must be provided")
	}

	normalizeNameservers, err := validateAndFormatNameservers(nameservers)
	if err != nil {
		return nil, err
	}

	resolvers := make([]Lookup, len(normalizeNameservers))
	for i, ns := range normalizeNameservers {
		resolvers[i] = newNetResolver(ns, timeout)
	}

	r := &NameserverResolver{
		maxRetries: maxRetries,
		resolvers:  resolvers,
		timeout:    timeout,
	}

	return r, nil
}

// NewSystemResolver creates a NameserverResolver that uses the system's DNS resolver
func NewSystemResolver(timeout time.Duration, maxRetries int) *NameserverResolver {
	return &NameserverResolver{
		maxRetries: maxRetries,
		resolvers:  []Lookup{newSystemNetResolver(timeout)},
		timeout:    timeout,
	}
}

// Resolve performs a forward DNS lookup (hostname to IP) using the configured nameservers.
// It returns the first IPv4 address found.
// If no IPv4 address is found, it returns the first IP address found.
func (r *NameserverResolver) Resolve(ctx context.Context, hostname string) ([]string, error) {
	return r.lookupWithNameservers(ctx, func(fnCtx context.Context, resolver Lookup) ([]string, error) {
		ips, err := resolver.LookupIP(fnCtx, "ip", hostname)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, ErrNoResolution
		}

		// convert result to string
		ipStrings := make([]string, len(ips))
		for i, ip := range ips {
			ipStrings[i] = ip.String()
		}

		return ipStrings, nil
	})
}

// Reverse performs a reverse DNS lookup (IP to hostname) using the configured nameservers.
// It returns the first hostname found.
func (r *NameserverResolver) Reverse(ctx context.Context, ip string) ([]string, error) {
	return r.lookupWithNameservers(ctx, func(fnCtx context.Context, resolver Lookup) ([]string, error) {
		hostnames, err := resolver.LookupAddr(fnCtx, ip)
		if err != nil && hostnames == nil {
			return nil, err
		}

		// hostname(s) was found but was filtered because of malformed DNS records
		if len(hostnames) == 0 {
			return nil, ErrNoResolution
		}

		// normalize result
		nHostnames := make([]string, len(hostnames))
		for i, hostname := range hostnames {
			nHostnames[i] = NormalizeHostname(hostname)
		}

		return nHostnames, nil
	})
}

func (r *NameserverResolver) Close() error {
	r.resolvers = nil
	return nil
}

func newExponentialBackOff() *backoff.ExponentialBackOff {
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
// The lookupFn is a function that performs the actual DNS lookup.
// It retries the lookup with exponential backoff on retryable failures, e.g., timeout and network temporary errors.
// No resolution is a valid result that no need to retry.
// If all nameservers fail, the last error is returned.
func (r *NameserverResolver) lookupWithNameservers(
	ctx context.Context,
	lookupFn func(fnCtx context.Context, resolver Lookup) ([]string, error),
) ([]string, error) {
	var lastErr error

	for _, resolver := range r.resolvers {
		expBackOff := newExponentialBackOff()

		for attempt := 0; attempt <= r.maxRetries; attempt++ {
			result, err := func() ([]string, error) {
				lookupCtx, cancel := context.WithTimeout(ctx, r.timeout)
				defer cancel()
				return lookupFn(lookupCtx, resolver)
			}()

			// Successful resolution
			if err == nil {
				return result, nil
			}

			// No resolution is a valid result
			if errors.Is(err, ErrNoResolution) {
				return nil, err
			}

			lastErr = err

			// The hostname was not found (NXDOMAIN), we can skip retrying
			if dnsErr := new(net.DNSError); errors.As(err, &dnsErr) && dnsErr.IsNotFound {
				return nil, ErrNoResolution
			}

			// Encounter non retryable error, skip retrying and move to the next nameserver
			if opErr := new(net.OpError); errors.As(err, &opErr) {
				if !opErr.Temporary() && !opErr.Timeout() {
					lastErr = ErrNSPermanentFailure
					break
				}
			} else if netErr := (net.Error)(nil); errors.As(err, &netErr) {
				if !netErr.Timeout() {
					lastErr = ErrNSPermanentFailure
					break
				}
			}

			// Sleep for retry
			if attempt < r.maxRetries {
				sleepDuration := expBackOff.NextBackOff()

				select {
				case <-time.After(sleepDuration): // Backoff
				case <-ctx.Done(): // Context cancelled during lookup retry
					return nil, ctx.Err()
				}
			}
		}
	}
	return nil, lastErr
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
		host, port, err := net.SplitHostPort(ns)
		if err != nil {
			nns = net.JoinHostPort(ns, "53")
			host, port, err = net.SplitHostPort(nns)
			if err != nil {
				return nil, fmt.Errorf("invalid nameserver address: %s", ns)
			}
		}

		// Validate address
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
