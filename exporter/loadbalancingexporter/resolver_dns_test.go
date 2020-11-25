// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadbalancingexporter

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestInitialDNSResolution(t *testing.T) {
	// prepare
	res, err := newDNSResolver(zap.NewNop(), "service-1", "")
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
	res.start(context.Background())
	defer res.shutdown(context.Background())

	// verify
	assert.Len(t, resolved, 3)
	for i, value := range []string{"127.0.0.1", "127.0.0.2", "[::1]"} {
		assert.Equal(t, value, resolved[i])
	}
}

func TestInitialDNSResolutionWithPort(t *testing.T) {
	// prepare
	res, err := newDNSResolver(zap.NewNop(), "service-1", "55690")
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
	res.start(context.Background())
	defer res.shutdown(context.Background())

	// verify
	assert.Len(t, resolved, 3)
	for i, value := range []string{"127.0.0.1:55690", "127.0.0.2:55690", "[::1]:55690"} {
		assert.Equal(t, value, resolved[i])
	}
}

func TestErrNoHostname(t *testing.T) {
	// test
	res, err := newDNSResolver(zap.NewNop(), "", "")

	// verify
	assert.Nil(t, res)
	assert.Equal(t, errNoHostname, err)
}

func TestCantResolve(t *testing.T) {
	// prepare
	res, err := newDNSResolver(zap.NewNop(), "service-1", "")
	require.NoError(t, err)

	expectedErr := errors.New("some expected error")
	res.resolver = &mockDNSResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return nil, expectedErr
		},
	}

	// test
	err = res.start(context.Background())

	// verify
	assert.Equal(t, expectedErr, err)
}

func TestOnChange(t *testing.T) {
	// prepare
	res, err := newDNSResolver(zap.NewNop(), "service-1", "")
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
	counter := 0
	res.onChange(func(endpoints []string) {
		counter++
	})
	res.start(context.Background())
	defer res.shutdown(context.Background())
	require.Equal(t, 1, counter)

	// now, we run it with the same IPs being resolved, which shouldn't trigger a onChange call
	res.resolve(context.Background())
	require.Equal(t, 1, counter)

	// change what the resolver will resolve and trigger a resolution
	resolve = []net.IPAddr{
		{IP: net.IPv4(127, 0, 0, 2)},
		{IP: net.IPv4(127, 0, 0, 3)},
	}
	res.resolve(context.Background())
	assert.Equal(t, 2, counter)
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
	res, err := newDNSResolver(zap.NewNop(), "service-1", "")
	require.NoError(t, err)

	counter := 0
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
				counter++
			}()
			// for second call, return the second result
			if counter == 2 {
				return resolve[1], nil
			}
			// for subsequent calls, return the last result, because we need more two periodic results
			// to confirm that it works as expected.
			if counter >= 3 {
				return resolve[2], nil
			}

			// for the first call, return the first result
			return resolve[0], nil
		},
	}
	res.resInterval = 10 * time.Millisecond

	wg := sync.WaitGroup{}
	res.onChange(func(backends []string) {
		wg.Done()
	})

	// test
	wg.Add(3)
	res.start(context.Background())
	defer res.shutdown(context.Background())

	// wait for three resolutions: from the start, and two periodic resolutions
	wg.Wait()

	// verify
	assert.GreaterOrEqual(t, counter, 3)
	assert.Len(t, res.endpoints, 3)
}

func TestPeriodicallyResolveFailure(t *testing.T) {
	// prepare
	res, err := newDNSResolver(zap.NewNop(), "service-1", "")
	require.NoError(t, err)

	expectedErr := errors.New("some expected error")
	wg := sync.WaitGroup{}
	counter := 0
	resolve := []net.IPAddr{{IP: net.IPv4(127, 0, 0, 1)}}
	res.resolver = &mockDNSResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			counter++

			// count down at most two times
			if counter <= 2 {
				wg.Done()
			}

			// for subsequent calls, return the error
			if counter >= 2 {
				return nil, expectedErr
			}

			// for the first call, return the first result
			return resolve, nil
		},
	}
	res.resInterval = 10 * time.Millisecond

	// test
	wg.Add(2)
	res.start(context.Background())
	defer res.shutdown(context.Background())

	// wait for two resolutions: from the start, and one periodic
	wg.Wait()

	// verify
	assert.GreaterOrEqual(t, 2, counter)
	assert.Len(t, res.endpoints, 1) // no change to the list of endpoints
}

func TestShutdownClearsCallbacks(t *testing.T) {
	// prepare
	res, err := newDNSResolver(zap.NewNop(), "service-1", "")
	require.NoError(t, err)

	res.resolver = &mockDNSResolver{}
	res.onChange(func(s []string) {})
	res.start(context.Background())

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

var _ netResolver = (*mockDNSResolver)(nil)

type mockDNSResolver struct {
	net.Resolver
	onLookupIPAddr func(context.Context, string) ([]net.IPAddr, error)
}

func (m *mockDNSResolver) LookupIPAddr(ctx context.Context, hostname string) ([]net.IPAddr, error) {
	if m.onLookupIPAddr != nil {
		return m.onLookupIPAddr(ctx, hostname)
	}
	return nil, nil
}
