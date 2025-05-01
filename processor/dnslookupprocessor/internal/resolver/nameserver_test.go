// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var (
	testTimeout = 100 * time.Millisecond
	testCtx     = context.Background()
)

// MockNetResolver implements the Lookup interface for testing
type MockNetResolver struct {
	mock.Mock
}

func (m *MockNetResolver) LookupIP(ctx context.Context, network, host string) ([]net.IP, error) {
	args := m.Called(ctx, network, host)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]net.IP), args.Error(1)
}

func (m *MockNetResolver) LookupAddr(ctx context.Context, addr string) ([]string, error) {
	args := m.Called(ctx, addr)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func TestNewNameserverResolver(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name        string
		nameservers []string
		expectError bool
	}{
		{
			name:        "valid nameservers",
			nameservers: []string{"8.8.8.8", "1.1.1.1:53"},
			expectError: false,
		},
		{
			name:        "empty nameservers",
			nameservers: []string{},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver, err := NewNameserverResolver(tc.nameservers, testTimeout, 3, logger)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, resolver)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resolver)
				assert.Len(t, tc.nameservers, len(resolver.resolvers))
			}
		})
	}
}

func TestNameserverResolver_Resolve(t *testing.T) {
	logger := zaptest.NewLogger(t)

	testCases := []struct {
		name           string
		hostname       string
		mockResolvers  []*MockNetResolver
		nameservers    []string
		expectedResult string
		expectError    bool
		expectedError  error
		maxRetries     int
	}{
		{
			name:     "successful resolution with IPv4",
			hostname: "example.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					ipv4 := net.ParseIP("192.168.1.1")
					ipv6 := net.ParseIP("2001:db8::1")

					m.On("LookupIP", mock.Anything, "ip", "example.com").
						Return([]net.IP{ipv6, ipv4}, nil)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			expectedResult: "192.168.1.1",
			expectError:    false,
			expectedError:  nil,
			maxRetries:     0,
		},
		{
			name:     "only IPv6 available",
			hostname: "ipv6only.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					ipv6 := net.ParseIP("2001:db8::1")

					m.On("LookupIP", mock.Anything, "ip", "ipv6only.com").
						Return([]net.IP{ipv6}, nil)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			expectedResult: "2001:db8::1",
			expectError:    false,
			expectedError:  nil,
			maxRetries:     0,
		},
		{
			name:     "no IPs returned",
			hostname: "empty.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					m.On("LookupIP", mock.Anything, "ip", "empty.com").
						Return([]net.IP{}, nil)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			expectedResult: "",
			expectError:    true,
			expectedError:  ErrNoResolution,
			maxRetries:     0,
		},
		{
			name:     "DNS not found error",
			hostname: "notfound.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					dnsErr := &net.DNSError{
						Err:         "no such host",
						Name:        "notfound.com",
						IsNotFound:  true,
						IsTemporary: false,
					}
					m.On("LookupIP", mock.Anything, "ip", "notfound.com").
						Return([]net.IP(nil), dnsErr)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			expectedResult: "",
			expectError:    true,
			expectedError:  ErrNoResolution,
			maxRetries:     0,
		},
		{
			name:     "temporary error with retry success",
			hostname: "retry.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					tempErr := &net.DNSError{
						Err:         "temporary failure",
						Name:        "retry.com",
						IsTemporary: true,
					}

					// First call fails with temporary error
					firstCall := m.On("LookupIP", mock.Anything, "ip", "retry.com").
						Return([]net.IP(nil), tempErr).Once()

					// Second call succeeds
					ipv4 := net.ParseIP("10.0.0.1")
					m.On("LookupIP", mock.Anything, "ip", "retry.com").
						Return([]net.IP{ipv4}, nil).NotBefore(firstCall)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			expectedResult: "10.0.0.1",
			expectError:    false,
			expectedError:  nil,
			maxRetries:     3,
		},
		{
			name:     "timeout error with retry failure",
			hostname: "timeout.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					timeoutErr := &net.DNSError{
						Err:       "i/o timeout",
						Name:      "timeout.com",
						IsTimeout: true,
					}

					// Always fail with timeout
					m.On("LookupIP", mock.Anything, "ip", "timeout.com").
						Return([]net.IP(nil), timeoutErr)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			expectedResult: "",
			expectError:    true,
			maxRetries:     2,
		},
		{
			name:     "fallback to second nameserver",
			hostname: "fallback.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					timeoutErr := &net.DNSError{
						Err:       "i/o timeout",
						Name:      "fallback.com",
						IsTimeout: true,
					}

					// First resolver always fails
					m.On("LookupIP", mock.Anything, "ip", "fallback.com").
						Return([]net.IP(nil), timeoutErr)
					return m
				}(),
				func() *MockNetResolver {
					m := new(MockNetResolver)
					// Second resolver succeeds
					ipv4 := net.ParseIP("10.1.1.1")
					m.On("LookupIP", mock.Anything, "ip", "fallback.com").
						Return([]net.IP{ipv4}, nil)
					return m
				}(),
			},
			nameservers:    []string{"test-server1", "test-server2"},
			expectedResult: "10.1.1.1",
			expectError:    false,
			expectedError:  nil,
			maxRetries:     1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockResolvers := make([]Lookup, len(tc.mockResolvers))
			for i, mockResolver := range tc.mockResolvers {
				mockResolvers[i] = mockResolver
			}

			nsResolver := &NameserverResolver{
				name:        "test",
				nameservers: tc.nameservers,
				maxRetries:  tc.maxRetries,
				resolvers:   mockResolvers,
				timeout:     testTimeout,
				logger:      logger,
			}

			result, err := nsResolver.Resolve(testCtx, tc.hostname)

			if tc.expectError {
				assert.Error(t, err)
				if tc.expectedError != nil {
					assert.Equal(t, tc.expectedError, err)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}

			for _, mockResolver := range tc.mockResolvers {
				mockResolver.AssertExpectations(t)
			}
		})
	}
}

func TestNameserverResolver_Reverse(t *testing.T) {
	logger := zaptest.NewLogger(t)

	testCases := []struct {
		name           string
		ip             string
		mockResolvers  []*MockNetResolver
		nameservers    []string
		expectedResult string
		expectError    bool
		expectedError  error
		maxRetries     int
	}{
		{
			name: "successful reverse resolution",
			ip:   "192.168.1.1",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					m.On("LookupAddr", mock.Anything, "192.168.1.1").
						Return([]string{"example.com."}, nil)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			expectedResult: "example.com",
			expectError:    false,
			expectedError:  nil,
			maxRetries:     0,
		},
		{
			name: "multiple reverse resolutions",
			ip:   "192.168.1.1",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					m.On("LookupAddr", mock.Anything, "192.168.1.1").
						Return([]string{"example.com.", "another-example.com."}, nil)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			expectedResult: "example.com",
			expectError:    false,
			expectedError:  nil,
			maxRetries:     0,
		},
		{
			name: "no hostnames returned",
			ip:   "10.0.0.1",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					m.On("LookupAddr", mock.Anything, "10.0.0.1").
						Return([]string{}, nil)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			expectedResult: "",
			expectError:    true,
			expectedError:  ErrNoResolution,
			maxRetries:     0,
		},
		{
			name: "DNS error",
			ip:   "172.16.0.1",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					dnsErr := &net.DNSError{
						Err:         "no such host",
						Name:        "172.16.0.1",
						IsNotFound:  true,
						IsTemporary: false,
					}
					m.On("LookupAddr", mock.Anything, "172.16.0.1").
						Return([]string(nil), dnsErr)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			expectedResult: "",
			expectError:    true,
			expectedError:  ErrNoResolution,
			maxRetries:     0,
		},
		{
			name: "malformed DNS records",
			ip:   "172.16.0.1",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					dnsErr := &net.DNSError{
						Err:  "DNS response contained records which contain invalid names",
						Name: "172.16.0.1",
					}
					m.On("LookupAddr", mock.Anything, "172.16.0.1").
						Return([]string{"example.com."}, dnsErr)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			expectedResult: "example.com",
			expectError:    false,
			maxRetries:     0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolvers := make([]Lookup, len(tc.mockResolvers))
			for i, mockResolver := range tc.mockResolvers {
				resolvers[i] = mockResolver
			}

			resolver := &NameserverResolver{
				name:        "test",
				nameservers: tc.nameservers,
				maxRetries:  tc.maxRetries,
				resolvers:   resolvers,
				timeout:     testTimeout,
				logger:      logger,
			}

			result, err := resolver.Reverse(testCtx, tc.ip)

			if tc.expectError {
				assert.Error(t, err)
				if tc.expectedError != nil {
					assert.Equal(t, tc.expectedError, err)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}

			for _, mockResolver := range tc.mockResolvers {
				mockResolver.AssertExpectations(t)
			}
		})
	}
}

func TestNormalizeNameserverAddresses(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		expected    []string
		expectError bool
	}{
		{
			name:        "IP without port",
			input:       []string{"8.8.8.8"},
			expected:    []string{"8.8.8.8:53"},
			expectError: false,
		},
		{
			name:        "IP with port",
			input:       []string{"8.8.8.8:53"},
			expected:    []string{"8.8.8.8:53"},
			expectError: false,
		},
		{
			name:        "mixed formats",
			input:       []string{"8.8.8.8", "1.1.1.1:53"},
			expected:    []string{"8.8.8.8:53", "1.1.1.1:53"},
			expectError: false,
		},
		{
			name:        "hostname without port",
			input:       []string{"dns-server.net"},
			expected:    []string{"dns-server.net:53"},
			expectError: false,
		},
		{
			name:        "invalid address",
			input:       []string{"invalid:host:port"},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := validateAndFormatNameservers(tc.input)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestNameserverResolver_Close(t *testing.T) {
	logger := zaptest.NewLogger(t)

	resolver, err := NewNameserverResolver([]string{"8.8.8.8"}, testTimeout, 1, logger)
	require.NoError(t, err)
	assert.Len(t, resolver.resolvers, 1)
	assert.NotNil(t, resolver.resolvers[0])

	err = resolver.Close()
	assert.NoError(t, err)
	assert.Nil(t, resolver.resolvers)
}
