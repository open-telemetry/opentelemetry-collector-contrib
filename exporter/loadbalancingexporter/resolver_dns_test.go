// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestInitialDNSResolution(t *testing.T) {
	// prepare
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "service-1", "", DNSModeStandard, 5*time.Second, 1*time.Second, tb)
	require.NoError(t, err)

	res.resolver = &mockDNSResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{
				{IP: net.IPv4(127, 0, 0, 1)},
				{IP: net.IPv4(127, 0, 0, 2)},
				{IP: net.IPv6loopback},
			}, nil
		},
	}

	// test
	var resolved []string
	res.onChange(func(endpoints []string) {
		resolved = endpoints
	})
	require.NoError(t, res.start(t.Context()))
	defer func() {
		require.NoError(t, res.shutdown(t.Context()))
	}()

	// verify
	assert.Len(t, resolved, 3)
	for i, value := range []string{"127.0.0.1", "127.0.0.2", "[::1]"} {
		assert.Equal(t, value, resolved[i])
	}
}

func TestInitialDNSResolutionWithPort(t *testing.T) {
	// prepare
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "service-1", "55690", DNSModeStandard, 5*time.Second, 1*time.Second, tb)
	require.NoError(t, err)

	res.resolver = &mockDNSResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{
				{IP: net.IPv4(127, 0, 0, 1)},
				{IP: net.IPv4(127, 0, 0, 2)},
				{IP: net.IPv6loopback},
			}, nil
		},
	}

	// test
	var resolved []string
	res.onChange(func(endpoints []string) {
		resolved = endpoints
	})
	require.NoError(t, res.start(t.Context()))
	defer func() {
		require.NoError(t, res.shutdown(t.Context()))
	}()

	// verify
	assert.Len(t, resolved, 3)
	for i, value := range []string{"127.0.0.1:55690", "127.0.0.2:55690", "[::1]:55690"} {
		assert.Equal(t, value, resolved[i])
	}
}

func TestErrNoHostname(t *testing.T) {
	// test
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "", "", DNSModeStandard, 5*time.Second, 1*time.Second, tb)

	// verify
	assert.Nil(t, res)
	assert.Equal(t, errNoHostname, err)
}

func TestCantResolve(t *testing.T) {
	// prepare
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "service-1", "", DNSModeStandard, 5*time.Second, 1*time.Second, tb)
	require.NoError(t, err)

	expectedErr := errors.New("some expected error")
	res.resolver = &mockDNSResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return nil, expectedErr
		},
	}

	// test
	require.NoError(t, res.start(t.Context()))

	// verify
	assert.NoError(t, err)
	assert.NoError(t, res.shutdown(t.Context()))
}

func TestOnChange(t *testing.T) {
	// prepare
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "service-1", "", DNSModeStandard, 5*time.Second, 1*time.Second, tb)
	require.NoError(t, err)

	resolve := []net.IPAddr{
		{IP: net.IPv4(127, 0, 0, 1)},
	}
	res.resolver = &mockDNSResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return resolve, nil
		},
	}

	// test
	counter := &atomic.Int64{}
	res.onChange(func([]string) {
		counter.Add(1)
	})
	require.NoError(t, res.start(t.Context()))
	defer func() {
		require.NoError(t, res.shutdown(t.Context()))
	}()
	require.Equal(t, int64(1), counter.Load())

	// now, we run it with the same IPs being resolved, which shouldn't trigger a onChange call
	_, err = res.resolve(t.Context())
	require.NoError(t, err)
	require.Equal(t, int64(1), counter.Load())

	// change what the resolver will resolve and trigger a resolution
	resolve = []net.IPAddr{
		{IP: net.IPv4(127, 0, 0, 2)},
		{IP: net.IPv4(127, 0, 0, 3)},
	}
	_, err = res.resolve(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(2), counter.Load())
}

func TestEqualStringSlice(t *testing.T) {
	for _, tt := range []struct {
		source    []string
		candidate []string
		expected  bool
	}{
		{
			[]string{"endpoint-1"},
			[]string{"endpoint-1"},
			true,
		},
		{
			[]string{"endpoint-1", "endpoint-2"},
			[]string{"endpoint-1"},
			false,
		},
		{
			[]string{"endpoint-1"},
			[]string{"endpoint-2"},
			false,
		},
	} {
		res := equalStringSlice(tt.source, tt.candidate)
		assert.Equal(t, tt.expected, res)
	}
}

func TestPeriodicallyResolve(t *testing.T) {
	// prepare
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "service-1", "", DNSModeStandard, 10*time.Millisecond, 1*time.Second, tb)
	require.NoError(t, err)

	counter := &atomic.Int64{}
	resolve := [][]net.IPAddr{
		{
			{IP: net.IPv4(127, 0, 0, 1)},
		}, {
			{IP: net.IPv4(127, 0, 0, 1)},
			{IP: net.IPv4(127, 0, 0, 2)},
		}, {
			{IP: net.IPv4(127, 0, 0, 1)},
			{IP: net.IPv4(127, 0, 0, 2)},
			{IP: net.IPv4(127, 0, 0, 3)},
		},
	}
	res.resolver = &mockDNSResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			defer func() {
				counter.Add(1)
			}()
			// for second call, return the second result
			if counter.Load() == 2 {
				return resolve[1], nil
			}
			// for subsequent calls, return the last result, because we need more two periodic results
			// to confirm that it works as expected.
			if counter.Load() >= 3 {
				return resolve[2], nil
			}

			// for the first call, return the first result
			return resolve[0], nil
		},
	}

	wg := sync.WaitGroup{}
	res.onChange(func([]string) {
		wg.Done()
	})

	// test
	wg.Add(3)
	require.NoError(t, res.start(t.Context()))
	defer func() {
		require.NoError(t, res.shutdown(t.Context()))
	}()

	// wait for three resolutions: from the start, and two periodic resolutions
	wg.Wait()

	// verify
	assert.GreaterOrEqual(t, counter.Load(), int64(3))
	assert.Len(t, res.endpoints, 3)
}

func TestPeriodicallyResolveFailure(t *testing.T) {
	// prepare
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "service-1", "", DNSModeStandard, 10*time.Millisecond, 1*time.Second, tb)
	require.NoError(t, err)

	expectedErr := errors.New("some expected error")
	wg := sync.WaitGroup{}
	counter := &atomic.Int64{}
	resolve := []net.IPAddr{{IP: net.IPv4(127, 0, 0, 1)}}
	res.resolver = &mockDNSResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			counter.Add(1)

			// count down at most two times
			if counter.Load() <= 2 {
				wg.Done()
			}

			// for subsequent calls, return the error
			if counter.Load() >= 2 {
				return nil, expectedErr
			}

			// for the first call, return the first result
			return resolve, nil
		},
	}

	// test
	wg.Add(2)
	require.NoError(t, res.start(t.Context()))
	defer func() {
		require.NoError(t, res.shutdown(t.Context()))
	}()

	// wait for two resolutions: from the start, and one periodic
	wg.Wait()

	// verify
	assert.GreaterOrEqual(t, counter.Load(), int64(2))
	assert.Len(t, res.endpoints, 1) // no change to the list of endpoints
}

func TestShutdownClearsCallbacks(t *testing.T) {
	// prepare
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "service-1", "", DNSModeStandard, 5*time.Second, 1*time.Second, tb)
	require.NoError(t, err)

	res.resolver = &mockDNSResolver{}
	res.onChange(func([]string) {})
	require.NoError(t, res.start(t.Context()))

	// sanity check
	require.Len(t, res.onChangeCallbacks, 1)

	// test
	err = res.shutdown(t.Context())

	// verify
	assert.NoError(t, err)
	assert.Empty(t, res.onChangeCallbacks)

	// check that we can add a new onChange before a new start
	res.onChange(func([]string) {})
	assert.Len(t, res.onChangeCallbacks, 1)
}

var _ netResolver = (*mockDNSResolver)(nil)

type mockDNSResolver struct {
	net.Resolver
	onLookupIPAddr func(context.Context, string) ([]net.IPAddr, error)
	onLookupSRV    func(context.Context, string, string, string) (string, []*net.SRV, error)
}

func (m *mockDNSResolver) LookupIPAddr(ctx context.Context, hostname string) ([]net.IPAddr, error) {
	if m.onLookupIPAddr != nil {
		return m.onLookupIPAddr(ctx, hostname)
	}
	return nil, nil
}

func (m *mockDNSResolver) LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error) {
	if m.onLookupSRV != nil {
		return m.onLookupSRV(ctx, service, proto, name)
	}
	return "", nil, nil
}

func TestDNSSRVResolution(t *testing.T) {
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "_svc._tcp.example.org", "", DNSModeSRV, 5*time.Second, 1*time.Second, tb)
	require.NoError(t, err)

	res.resolver = &mockDNSResolver{
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			// emulate two SRV records
			return "", []*net.SRV{
				{Target: "host1.example.org.", Port: 1000},
				{Target: "host2.example.org.", Port: 2000},
			}, nil
		},
		onLookupIPAddr: func(_ context.Context, host string) ([]net.IPAddr, error) {
			if host == "host1.example.org." {
				return []net.IPAddr{{IP: net.IPv4(10, 0, 0, 1)}}, nil
			}
			if host == "host2.example.org." {
				return []net.IPAddr{{IP: net.IPv6loopback}}, nil
			}
			return nil, nil
		},
	}

	var resolved []string
	res.onChange(func(endpoints []string) {
		resolved = endpoints
	})
	require.NoError(t, res.start(t.Context()))
	defer func() { require.NoError(t, res.shutdown(t.Context())) }()

	assert.Len(t, resolved, 2)
	assert.Equal(t, "10.0.0.1:1000", resolved[0])
	assert.Equal(t, "[::1]:2000", resolved[1])
}

func TestDNSSRVNoHostname(t *testing.T) {
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "", "", DNSModeSRV, 5*time.Second, 1*time.Second, tb)

	assert.Nil(t, res)
	assert.Equal(t, errNoHostname, err)
}

func TestDNSSRVLookupError(t *testing.T) {
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "_svc._tcp.example.org", "", DNSModeSRV, 5*time.Second, 1*time.Second, tb)
	require.NoError(t, err)

	expectedErr := errors.New("some expected error")
	res.resolver = &mockDNSResolver{
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			return "", nil, expectedErr
		},
	}

	// starting should not fail
	require.NoError(t, res.start(t.Context()))
	require.NoError(t, res.shutdown(t.Context()))
}

func TestDNSSRVPartialIPLookupFailure(t *testing.T) {
	// When one SRV target's A/AAAA lookup fails, the other targets should still be resolved.
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "_svc._tcp.example.org", "", DNSModeSRV, 5*time.Second, 1*time.Second, tb)
	require.NoError(t, err)

	res.resolver = &mockDNSResolver{
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			return "", []*net.SRV{
				{Target: "healthy.example.org.", Port: 8080},
				{Target: "broken.example.org.", Port: 9090},
			}, nil
		},
		onLookupIPAddr: func(_ context.Context, host string) ([]net.IPAddr, error) {
			if host == "healthy.example.org." {
				return []net.IPAddr{{IP: net.IPv4(10, 0, 0, 1)}}, nil
			}
			return nil, errors.New("lookup failed")
		},
	}

	var resolved []string
	res.onChange(func(endpoints []string) {
		resolved = endpoints
	})
	require.NoError(t, res.start(t.Context()))
	defer func() { require.NoError(t, res.shutdown(t.Context())) }()

	// only the healthy target should appear
	assert.Len(t, resolved, 1)
	assert.Equal(t, "10.0.0.1:8080", resolved[0])
}

func TestDNSSRVTargetWithMultipleIPs(t *testing.T) {
	// A single SRV target that resolves to multiple A/AAAA records (e.g. round-robin DNS).
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "_svc._tcp.example.org", "", DNSModeSRV, 5*time.Second, 1*time.Second, tb)
	require.NoError(t, err)

	res.resolver = &mockDNSResolver{
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			return "", []*net.SRV{
				{Target: "multi.example.org.", Port: 4317},
			}, nil
		},
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{
				{IP: net.IPv4(10, 0, 0, 1)},
				{IP: net.IPv4(10, 0, 0, 2)},
				{IP: net.IPv6loopback},
			}, nil
		},
	}

	var resolved []string
	res.onChange(func(endpoints []string) {
		resolved = endpoints
	})
	require.NoError(t, res.start(t.Context()))
	defer func() { require.NoError(t, res.shutdown(t.Context())) }()

	assert.Len(t, resolved, 3)
	// sorted order
	assert.Equal(t, "10.0.0.1:4317", resolved[0])
	assert.Equal(t, "10.0.0.2:4317", resolved[1])
	assert.Equal(t, "[::1]:4317", resolved[2])
}

func TestDNSSRVEmptyResult(t *testing.T) {
	// LookupSRV succeeds but returns zero SRV records.
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "_svc._tcp.example.org", "", DNSModeSRV, 5*time.Second, 1*time.Second, tb)
	require.NoError(t, err)

	res.resolver = &mockDNSResolver{
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			return "", []*net.SRV{}, nil
		},
	}

	var resolved []string
	res.onChange(func(endpoints []string) {
		resolved = endpoints
	})
	require.NoError(t, res.start(t.Context()))
	defer func() { require.NoError(t, res.shutdown(t.Context())) }()

	assert.Empty(t, resolved)
}

func TestDNSSRVOnChangeNotCalledOnSameResult(t *testing.T) {
	// Resolving the same SRV records twice should not trigger onChange a second time.
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "_svc._tcp.example.org", "", DNSModeSRV, 5*time.Second, 1*time.Second, tb)
	require.NoError(t, err)

	res.resolver = &mockDNSResolver{
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			return "", []*net.SRV{
				{Target: "host1.example.org.", Port: 4317},
			}, nil
		},
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.IPv4(10, 0, 0, 1)}}, nil
		},
	}

	counter := &atomic.Int64{}
	res.onChange(func([]string) {
		counter.Add(1)
	})
	require.NoError(t, res.start(t.Context()))
	defer func() { require.NoError(t, res.shutdown(t.Context())) }()
	require.Equal(t, int64(1), counter.Load())

	// resolve again with the same result — onChange should NOT fire
	_, err = res.resolve(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(1), counter.Load())

	// now change the result and resolve — onChange should fire
	res.resolver = &mockDNSResolver{
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			return "", []*net.SRV{
				{Target: "host1.example.org.", Port: 4317},
				{Target: "host2.example.org.", Port: 4318},
			}, nil
		},
		onLookupIPAddr: func(_ context.Context, host string) ([]net.IPAddr, error) {
			if host == "host1.example.org." {
				return []net.IPAddr{{IP: net.IPv4(10, 0, 0, 1)}}, nil
			}
			return []net.IPAddr{{IP: net.IPv4(10, 0, 0, 2)}}, nil
		},
	}
	_, err = res.resolve(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(2), counter.Load())
}

func TestDNSSRVPeriodicallyResolve(t *testing.T) {
	// Periodic resolution should pick up changes in SRV records over time.
	_, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "_svc._tcp.example.org", "", DNSModeSRV, 10*time.Millisecond, 1*time.Second, tb)
	require.NoError(t, err)

	counter := &atomic.Int64{}
	resolve := [][]*net.SRV{
		{{Target: "host1.example.org.", Port: 4317}},
		{{Target: "host1.example.org.", Port: 4317}, {Target: "host2.example.org.", Port: 4318}},
	}
	res.resolver = &mockDNSResolver{
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			idx := counter.Load()
			if int(idx) >= len(resolve) {
				return "", resolve[len(resolve)-1], nil
			}
			return "", resolve[idx], nil
		},
		onLookupIPAddr: func(_ context.Context, host string) ([]net.IPAddr, error) {
			if host == "host1.example.org." {
				return []net.IPAddr{{IP: net.IPv4(10, 0, 0, 1)}}, nil
			}
			return []net.IPAddr{{IP: net.IPv4(10, 0, 0, 2)}}, nil
		},
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	res.onChange(func([]string) {
		counter.Add(1)
		wg.Done()
	})

	require.NoError(t, res.start(t.Context()))
	defer func() { require.NoError(t, res.shutdown(t.Context())) }()

	// wait for initial + one periodic resolution that sees the changed list
	wg.Wait()

	assert.GreaterOrEqual(t, counter.Load(), int64(2))
	assert.Len(t, res.endpoints, 2)
}
