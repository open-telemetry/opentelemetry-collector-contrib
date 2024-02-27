// // Copyright The OpenTelemetry Authors
// // SPDX-License-Identifier: Apache-2.0

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

func TestInitialSRVResolution(t *testing.T) {
	// prepare
	res, err := newDNSSRVNOAResolver(zap.NewNop(), "_port._proto.service-1.ns.svc.cluster.local", 5*time.Second, 1*time.Second)
	require.NoError(t, err)

	res.resolver = &mockSRVResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{
				{IP: net.IPv4(12, 12, 12, 12)},
			}, nil
		},
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			return "notapplicable", []*net.SRV{
				{Target: "pod-0.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
				{Target: "pod-1.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
				{Target: "pod-2.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
			}, nil
		},
	}

	// test
	var resolved []string
	res.onChange(func(endpoints []string) {
		resolved = endpoints
	})
	require.NoError(t, res.start(context.Background()))
	defer func() {
		require.NoError(t, res.shutdown(context.Background()))
	}()

	// verify
	assert.Len(t, resolved, 3)
	for i, value := range []string{
		"pod-0.service-1.ns.svc.cluster.local:4317",
		"pod-1.service-1.ns.svc.cluster.local:4317",
		"pod-2.service-1.ns.svc.cluster.local:4317",
	} {
		assert.Equal(t, value, resolved[i])
	}
}

func TestErrNoSRVHostname(t *testing.T) {
	// test
	res, err := newDNSSRVNOAResolver(zap.NewNop(), "", 5*time.Second, 1*time.Second)

	// verify
	assert.Nil(t, res)
	assert.Equal(t, errNoSRV, err)
}

// This tests the case where a failure to resolve does not halt the resolver.
func TestFailureToResolve(t *testing.T) {
	// prepare
	res, err := newDNSSRVNOAResolver(zap.NewNop(), "_port._proto.service-1.ns.svc.cluster.local", 5*time.Second, 1*time.Second)
	require.NoError(t, err)

	expectedErr := errors.New("underlying resolution error")
	res.resolver = &mockSRVResolver{
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			return "", nil, expectedErr
		},
	}

	// test
	require.NoError(t, res.start(context.Background()))

	// verify
	assert.NoError(t, err)
}

func TestMultipleIPsOnATarget(t *testing.T) {
	// prepare
	res, err := newDNSSRVNOAResolver(zap.NewNop(), "_port._proto.service-1.ns.svc.cluster.local", 5*time.Second, 1*time.Second)
	require.NoError(t, err)

	res.resolver = &mockSRVResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{
				{IP: net.IPv4(12, 12, 12, 12)},
				{IP: net.IPv4(13, 13, 13, 13)},
			}, nil
		},
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			return "notapplicable", []*net.SRV{
				{Target: "pod-0.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
			}, nil
		},
	}

	// test
	require.NoError(t, res.start(context.Background()))

	// verify
	assert.Error(t, errNotSingleIP)
}

func TestSRVNoChangeAfterInitial(t *testing.T) {
	// prepare
	res, err := newDNSSRVNOAResolver(zap.NewNop(), "_port._proto.service-1.ns.svc.cluster.local", 5*time.Second, 1*time.Second)
	require.NoError(t, err)

	ipResolve := []net.IPAddr{
		{IP: net.IPv4(127, 0, 0, 1)},
	}
	res.resolver = &mockSRVResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return ipResolve, nil
		},
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			return "notapplicable", []*net.SRV{
				{Target: "pod-0.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
			}, nil
		},
	}

	// test
	counter := &atomic.Int64{}
	res.onChange(func(endpoints []string) {
		counter.Add(1)
	})
	require.NoError(t, res.start(context.Background()))
	defer func() {
		require.NoError(t, res.shutdown(context.Background()))
	}()
	require.Equal(t, int64(1), counter.Load())

	// now, we run it with the same IPs being resolved, which shouldn't trigger a onChange call
	_, err = res.resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), counter.Load())

	// change what the resolver will resolve and trigger a resolution
	ipResolve = []net.IPAddr{
		{IP: net.IPv4(127, 0, 0, 2)},
	}
	_, err = res.resolve(context.Background())
	require.NoError(t, err)
	assert.Greater(t, counter.Load(), int64(1))
}

func TestSameEndpointsDifferentIPs(t *testing.T) {
	// prepare
	res, err := newDNSSRVNOAResolver(zap.NewNop(), "_port._proto.service-1.ns.svc.cluster.local", 10*time.Millisecond, 1*time.Second)
	require.NoError(t, err)

	runCounter := &atomic.Int32{}
	resolve := map[string][][]net.IPAddr{
		"pod-0.service-1.ns.svc.cluster.local": {
			{{IP: net.IPv4(1, 1, 1, 1)}},
		},
		"pod-1.service-1.ns.svc.cluster.local": {
			{{IP: net.IPv4(2, 2, 2, 2)}},
		},
		"pod-2.service-1.ns.svc.cluster.local": {
			{{IP: net.IPv4(3, 3, 3, 3)}},
			{{IP: net.IPv4(4, 4, 4, 4)}},
		},
	}

	res.resolver = &mockSRVResolver{
		onLookupIPAddr: func(ctx context.Context, endpoint string) ([]net.IPAddr, error) {

			// pod-0 and pod-1 will always return the same IP
			if len(resolve[endpoint]) == 1 {
				return resolve[endpoint][0], nil
			}

			// for second call, act like pod-2 was restarted and the IP changed
			if runCounter.Load() >= 2 {
				return resolve[endpoint][1], nil
			}

			return resolve[endpoint][0], nil
		},
		// always return the same endpoints
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			defer func() {
				runCounter.Add(1)
			}()
			return "notapplicable", []*net.SRV{
				{Target: "pod-0.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
				{Target: "pod-1.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
				{Target: "pod-2.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
			}, nil
		},
	}

	wg := sync.WaitGroup{}
	changeCounter := &atomic.Int32{}
	res.onChange(func(backends []string) {
		changeCounter.Add(1)
		wg.Done()
	})

	// test
	wg.Add(3)
	require.NoError(t, res.start(context.Background()))
	defer func() {
		require.NoError(t, res.shutdown(context.Background()))
	}()

	// wait for three resolutions: from the start, and two periodic resolutions
	wg.Wait()

	// verify
	// Should be 2 since these are runs of resolving the SRV hostname
	assert.GreaterOrEqual(t, runCounter.Load(), int32(2))
	// Should be 3 changes. the initial resolution, one where we dropped pod-2,
	// and one where we added pod-2 back
	assert.Equal(t, int32(3), changeCounter.Load())
	assert.Len(t, res.endpoints, 3)
}

func TestSRVPeriodicallyResolve(t *testing.T) {
	// prepare
	res, err := newDNSSRVNOAResolver(zap.NewNop(), "_port._proto.service-1.ns.svc.cluster.local", 10*time.Millisecond, 1*time.Second)
	require.NoError(t, err)

	counter := &atomic.Int64{}
	resolve := [][]*net.SRV{
		{
			{Target: "pod-0.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
		},
		{
			{Target: "pod-0.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
			{Target: "pod-1.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
		},
		{
			{Target: "pod-0.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
			{Target: "pod-1.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
			{Target: "pod-2.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
		},
	}

	res.resolver = &mockSRVResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{
				{IP: net.IPv4(12, 12, 12, 12)},
			}, nil
		},
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			defer func() {
				counter.Add(1)
			}()
			// for second call, return the second result
			if counter.Load() == 2 {
				return "", resolve[1], nil
			}
			// for subsequent calls, return the last result, because we need more two periodic results
			// to confirm that it works as expected.
			if counter.Load() >= 3 {
				return "", resolve[2], nil
			}

			// for the first call, return the first result
			return "", resolve[0], nil
		},
	}

	wg := sync.WaitGroup{}
	res.onChange(func(backends []string) {
		wg.Done()
	})

	// test
	wg.Add(3)
	require.NoError(t, res.start(context.Background()))
	defer func() {
		require.NoError(t, res.shutdown(context.Background()))
	}()

	// wait for four resolutions: from the start, and three periodic resolutions
	wg.Wait()

	// verify
	assert.GreaterOrEqual(t, counter.Load(), int64(3))
	assert.Len(t, res.endpoints, 3)
}

func TestSRVPeriodicallyResolveFailure(t *testing.T) {
	// prepare
	res, err := newDNSSRVNOAResolver(zap.NewNop(), "_port._proto.service-1.ns.svc.cluster.local", 10*time.Millisecond, 1*time.Second)
	require.NoError(t, err)

	expectedErr := errors.New("some expected error")
	wg := sync.WaitGroup{}
	counter := &atomic.Int64{}
	resolve := []*net.SRV{
		{Target: "pod-0.service-1.ns.svc.cluster.local", Port: 4317, Priority: 0, Weight: 0},
	}
	res.resolver = &mockSRVResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{
				{IP: net.IPv4(12, 12, 12, 12)},
			}, nil
		},
		onLookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
			counter.Add(1)

			// count down at most two times
			if counter.Load() <= 2 {
				wg.Done()
			}

			// for subsequent calls, return the error
			if counter.Load() >= 2 {
				return "", nil, expectedErr
			}

			// for the first call, return the first result
			return "", resolve, nil
		},
	}

	// test
	wg.Add(2)
	require.NoError(t, res.start(context.Background()))
	defer func() {
		require.NoError(t, res.shutdown(context.Background()))
	}()

	// wait for two resolutions: from the start, and one periodic
	wg.Wait()

	// verify
	assert.GreaterOrEqual(t, counter.Load(), int64(2))
	assert.Len(t, res.endpoints, 1) // no change to the list of endpoints
}

func TestSRVShutdownClearsCallbacks(t *testing.T) {
	// prepare
	res, err := newDNSSRVNOAResolver(zap.NewNop(), "_port._proto.service-1.ns.svc.cluster.local", 2*time.Second, 1*time.Second)
	require.NoError(t, err)

	res.resolver = &mockSRVResolver{}
	res.onChange(func(s []string) {})
	require.NoError(t, res.start(context.Background()))

	// sanity check
	require.Len(t, res.onChangeCallbacks, 1)

	// test
	err = res.shutdown(context.Background())

	// verify
	assert.NoError(t, err)
	assert.Len(t, res.onChangeCallbacks, 0)

	// check that we can add a new onChange before a new start
	res.onChange(func(s []string) {})
	assert.Len(t, res.onChangeCallbacks, 1)
}

func TestParseSRVHostname(t *testing.T) {
	for _, tt := range []struct {
		input           string
		expectedService string
		expectedProto   string
		expectedName    string
		expectedErr     error
	}{
		// success case
		{
			input:           "_service._proto.name",
			expectedService: "service",
			expectedProto:   "proto",
			expectedName:    "name",
			expectedErr:     nil,
		},
	} {
		res, resErr := parseSRVHostname(tt.input)
		assert.Equal(t, tt.expectedService, res.service)
		assert.Equal(t, tt.expectedProto, res.proto)
		assert.Equal(t, tt.expectedName, res.name)
		assert.Equal(t, tt.expectedErr, resErr)
	}
}

func TestParseSRVHostnameFailure(t *testing.T) {
	for _, tt := range []struct {
		input       string
		expectedErr error
	}{
		// failure case
		{
			input:       "_service._proto",
			expectedErr: errBadSRV,
		},
	} {
		_, resErr := parseSRVHostname(tt.input)
		assert.Equal(t, tt.expectedErr, resErr)
	}
}

var _ netResolver = (*mockSRVResolver)(nil)

type mockSRVResolver struct {
	net.Resolver
	onLookupIPAddr func(context.Context, string) ([]net.IPAddr, error)
	onLookupSRV    func(context.Context, string, string, string) (string, []*net.SRV, error)
}

func (m *mockSRVResolver) LookupIPAddr(ctx context.Context, hostname string) ([]net.IPAddr, error) {
	if m.onLookupIPAddr != nil {
		return m.onLookupIPAddr(ctx, hostname)
	}
	return nil, nil
}

func (m *mockSRVResolver) LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error) {
	if m.onLookupSRV != nil {
		return m.onLookupSRV(ctx, service, proto, name)
	}
	return "", nil, nil
}
