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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialDNSResolution(t *testing.T) {
	// prepare
	res, err := newDNSResolver("service-1")
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

func TestErrNoHostname(t *testing.T) {
	// test
	res, err := newDNSResolver("")

	// verify
	assert.Nil(t, res)
	assert.Equal(t, errNoHostname, err)
}

func TestCantResolve(t *testing.T) {
	// prepare
	res, err := newDNSResolver("service-1")
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
	res, err := newDNSResolver("service-1")
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
