// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"net"
	"syscall"
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

func TestNewSystemResolver(t *testing.T) {
	logger := zaptest.NewLogger(t)

	resolver := NewSystemResolver(testTimeout, 3, logger)
	assert.NotNil(t, resolver)
}

func TestNameserverResolver_Resolve(t *testing.T) {
	logger := zaptest.NewLogger(t)

	testCases := []struct {
		name           string
		hostname       string
		mockResolvers  []*MockNetResolver
		nameservers    []string
		expectedResult []string
		expectError    bool
		expectedError  error
		maxRetries     int
	}{
		{
			name:     "successful all resolutions",
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
			maxRetries:     0,
			expectedResult: []string{"192.168.1.1", "2001:db8::1"},
			expectError:    false,
			expectedError:  nil,
		},
		{
			name:     "no IPs returned",
			hostname: "empty.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					m.On("LookupIP", mock.Anything, "ip", "empty.com").
						Return(nil, nil)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			maxRetries:     0,
			expectedResult: nil,
			expectError:    true,
			expectedError:  ErrNoResolution,
		},
		{
			name:     "NXDOMAIN from server",
			hostname: "notfound.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					dnsErr := &net.DNSError{
						IsNotFound:  true,
						IsTemporary: false,
					}
					m.On("LookupIP", mock.Anything, "ip", "notfound.com").
						Return(nil, dnsErr)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			maxRetries:     0,
			expectedResult: nil,
			expectError:    true,
			expectedError:  ErrNoResolution,
		},
		{
			name:     "temporary error with retry success",
			hostname: "retry.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					tempErr := &net.DNSError{
						Err:       "temporary failure",
						Name:      "retry.com",
						IsTimeout: true,
					}

					// First call fails with temporary error
					firstCall := m.On("LookupIP", mock.Anything, "ip", "retry.com").
						Return(nil, tempErr).Once()

					// Second call succeeds
					ipv4 := net.ParseIP("10.0.0.1")
					m.On("LookupIP", mock.Anything, "ip", "retry.com").
						Return([]net.IP{ipv4}, nil).NotBefore(firstCall)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			maxRetries:     3,
			expectedResult: []string{"10.0.0.1"},
			expectError:    false,
			expectedError:  nil,
		},
		{
			name:     "timeout error with retry failure",
			hostname: "timeout.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					timeoutErr := &net.DNSError{
						IsTimeout: true,
					}

					// Always fail with timeout
					m.On("LookupIP", mock.Anything, "ip", "timeout.com").
						Return(nil, timeoutErr)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			maxRetries:     2,
			expectedResult: nil,
			expectError:    true,
			expectedError: &net.DNSError{
				IsTimeout: true,
			},
		},
		{
			name:     "permanent connection refused error",
			hostname: "refused.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					opErr := &net.OpError{
						Err: syscall.ECONNREFUSED,
					}

					// Always fail with timeout
					m.On("LookupIP", mock.Anything, "ip", "refused.com").
						Return(nil, opErr)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			maxRetries:     2,
			expectedResult: nil,
			expectError:    true,
			expectedError:  ErrNSPermanentFailure,
		},
		{
			name:     "permanent DNS config error",
			hostname: "config-error.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					configErr := &net.DNSConfigError{}

					// Always fail with timeout
					m.On("LookupIP", mock.Anything, "ip", "config-error.com").
						Return([]net.IP(nil), configErr)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			maxRetries:     2,
			expectedResult: nil,
			expectError:    true,
			expectedError:  ErrNSPermanentFailure,
		},
		{
			name:     "fallback to second nameserver",
			hostname: "fallback.com",
			mockResolvers: []*MockNetResolver{
				func() *MockNetResolver {
					m := new(MockNetResolver)
					timeoutErr := &net.DNSError{
						IsTimeout: true,
					}
					// First resolver always fails
					m.On("LookupIP", mock.Anything, "ip", "fallback.com").
						Return(nil, timeoutErr)
					return m
				}(),
				func() *MockNetResolver {
					m := new(MockNetResolver)
					// Second resolver succeeds
					ip := net.ParseIP("10.1.1.1")
					m.On("LookupIP", mock.Anything, "ip", "fallback.com").
						Return([]net.IP{ip}, nil)
					return m
				}(),
			},
			nameservers:    []string{"test-server1", "test-server2"},
			maxRetries:     1,
			expectedResult: []string{"10.1.1.1"},
			expectError:    false,
			expectedError:  nil,
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
				assert.ElementsMatch(t, tc.expectedResult, result)
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
		maxRetries     int
		expectedResult []string
		expectError    bool
		expectedError  error
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
			maxRetries:     0,
			expectedResult: []string{"example.com"},
			expectError:    false,
			expectedError:  nil,
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
			maxRetries:     0,
			expectedResult: []string{"example.com", "another-example.com"},
			expectError:    false,
			expectedError:  nil,
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
			maxRetries:     0,
			expectedResult: nil,
			expectError:    true,
			expectedError:  ErrNoResolution,
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
						Return(nil, dnsErr)
					return m
				}(),
			},
			nameservers:    []string{"test-server"},
			maxRetries:     0,
			expectedResult: nil,
			expectError:    true,
			expectedError:  ErrNoResolution,
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
			maxRetries:     0,
			expectedResult: []string{"example.com"},
			expectError:    false,
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
				assert.ElementsMatch(t, tc.expectedResult, result)
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
			name:        "hostname with port",
			input:       []string{"dns-server.net:53"},
			expected:    []string{"dns-server.net:53"},
			expectError: false,
		},
		{
			name:        "invalid address",
			input:       []string{"invalid:host:port"},
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid port",
			input:       []string{"invalid:port"},
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
