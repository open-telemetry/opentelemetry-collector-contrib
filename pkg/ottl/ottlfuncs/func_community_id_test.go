// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestCommunityID(t *testing.T) {
	tests := []struct {
		name           string
		sourceIP       string
		destIP         string
		sourcePort     int64
		destPort       int64
		protocol       string
		seed           int64
		expected       string
		errorExpected  bool
		expectedErrMsg string
	}{
		{
			name:          "TCP IPv4 Example",
			sourceIP:      "1.2.3.4",
			destIP:        "5.6.7.8",
			sourcePort:    12345,
			destPort:      80,
			protocol:      "TCP",
			seed:          0,
			expected:      "1:0by3b/tE95hcOzccyt6d4kjgbZc=",
			errorExpected: false,
		},
		{
			name:          "TCP IPv4 Example with source and dest flipped",
			sourceIP:      "5.6.7.8",
			destIP:        "1.2.3.4",
			sourcePort:    80,
			destPort:      12345,
			protocol:      "TCP",
			seed:          0,
			expected:      "1:0by3b/tE95hcOzccyt6d4kjgbZc=",
			errorExpected: false,
		},
		{
			name:          "UDP IPv4 Example",
			sourceIP:      "192.168.1.1",
			destIP:        "10.10.10.10",
			sourcePort:    53,
			destPort:      53000,
			protocol:      "UDP",
			seed:          0,
			expected:      "1:gQ05/45srnixHs8V/2ejpkxhBwg=",
			errorExpected: false,
		},
		{
			name:          "ICMP Example",
			sourceIP:      "10.10.10.10",
			destIP:        "192.168.1.1",
			sourcePort:    53000,
			destPort:      53,
			protocol:      "ICMP",
			seed:          0,
			expected:      "1:9TGPWejPFuBlKrHuVcM3iaA8hf8=",
			errorExpected: false,
		},
		{
			name:          "RSVP Example",
			sourceIP:      "10.10.10.10",
			destIP:        "192.168.1.1",
			sourcePort:    53,
			destPort:      53000,
			protocol:      "RSVP",
			seed:          0,
			expected:      "1:rkXhRghBvx2yf3zgDnSHuJ35XaY=",
			errorExpected: false,
		},
		{
			name:          "ICMP6 Example",
			sourceIP:      "192.168.1.1",
			destIP:        "10.10.10.10",
			sourcePort:    53,
			destPort:      53000,
			protocol:      "ICMP6",
			seed:          0,
			expected:      "1:bchQ2SAoVf2Iq1XdGBXObLJ2i64=",
			errorExpected: false,
		},
		{
			name:          "SCTP Example",
			sourceIP:      "192.168.1.1",
			destIP:        "10.10.10.10",
			sourcePort:    53,
			destPort:      53000,
			protocol:      "SCTP",
			seed:          0,
			expected:      "1:zo1kfEyhghMaeU7tQDCE2G+ITQc=",
			errorExpected: false,
		},
		{
			name:          "TCP IPv6 Example",
			sourceIP:      "2001:db8::1",
			destIP:        "2001:db8::2",
			sourcePort:    8080,
			destPort:      443,
			protocol:      "TCP",
			seed:          0,
			expected:      "1:c5m26SNBLmvfaBQJNWKXZwUGGcM=",
			errorExpected: false,
		},
		{
			name:          "Custom Seed Example",
			sourceIP:      "192.168.1.1",
			destIP:        "192.168.1.2",
			sourcePort:    12345,
			destPort:      80,
			protocol:      "TCP",
			seed:          1234,
			expected:      "1:v1n8p4IZW9jXIJnFANLbRU2ahdU=",
			errorExpected: false,
		},
		{
			name:          "Normalized Direction Example",
			sourceIP:      "10.0.0.2", // Higher IP should be normalized
			destIP:        "10.0.0.1",
			sourcePort:    80,
			destPort:      12345,
			protocol:      "TCP",
			seed:          0,
			expected:      "1:CpuULklTENbGdRpvp7gNcQd5ZqA=",
			errorExpected: false,
		},
		{
			name:          "Same source and destinations",
			sourceIP:      "127.0.0.1",
			destIP:        "127.0.0.1",
			sourcePort:    8080,
			destPort:      8080,
			protocol:      "TCP",
			seed:          0,
			expected:      "1:sG2vk7rcZ12ZxPg+nnwljgdVEGw=",
			errorExpected: false,
		},
		{
			name:           "Invalid Source IP",
			sourceIP:       "invalid-ip",
			destIP:         "10.0.0.1",
			sourcePort:     80,
			destPort:       12345,
			protocol:       "TCP",
			seed:           0,
			errorExpected:  true,
			expectedErrMsg: "invalid source IP",
		},
		{
			name:           "Invalid Protocol",
			sourceIP:       "10.0.0.1",
			destIP:         "10.0.0.2",
			sourcePort:     80,
			destPort:       12345,
			protocol:       "UNKNOWN",
			seed:           0,
			errorExpected:  true,
			expectedErrMsg: "unsupported protocol",
		},
		{
			name:          "Different IP families",
			sourceIP:      "2001:db8::1",
			destIP:        "10.1.1.1",
			sourcePort:    8080,
			destPort:      443,
			protocol:      "TCP",
			seed:          0,
			expected:      "1:T1e3va6jTSWaSKkG3k4dnZq5VM8=",
			errorExpected: false,
		},
		{
			name:          "Same IP lengths with higher source port",
			sourceIP:      "10.1.1.1",
			destIP:        "10.1.1.2",
			sourcePort:    8080,
			destPort:      8079,
			protocol:      "TCP",
			seed:          0,
			expected:      "1:1fjNfFLjLTsO2WE9AY0TbTeSaBs=",
			errorExpected: false,
		},
		{
			name:          "Same IP lengths with higher dest port",
			sourceIP:      "10.1.1.2",
			destIP:        "10.1.1.1",
			sourcePort:    8079,
			destPort:      8080,
			protocol:      "TCP",
			seed:          0,
			expected:      "1:1fjNfFLjLTsO2WE9AY0TbTeSaBs=",
			errorExpected: false,
		},
		{
			name:          "Same IP with higher source port",
			sourceIP:      "10.1.1.2",
			destIP:        "10.1.1.2",
			sourcePort:    8081,
			destPort:      8080,
			protocol:      "TCP",
			seed:          0,
			expected:      "1:3byUvmBpOu8KHoTRr4eN5AjTQEU=",
			errorExpected: false,
		},
		{
			name:          "Same IP with higher dest port",
			sourceIP:      "10.1.1.2",
			destIP:        "10.1.1.2",
			sourcePort:    8080,
			destPort:      8081,
			protocol:      "TCP",
			seed:          0,
			expected:      "1:3byUvmBpOu8KHoTRr4eN5AjTQEU=",
			errorExpected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceIP := ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tt.sourceIP, nil
				},
			}
			destIP := ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tt.destIP, nil
				},
			}

			sourcePort := ottl.StandardIntGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tt.sourcePort, nil
				},
			}
			destPort := ottl.StandardIntGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tt.destPort, nil
				},
			}

			protocol := ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tt.protocol, nil
				},
			}

			seed := ottl.StandardIntGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tt.seed, nil
				},
			}

			protocolOpt := ottl.NewTestingOptional[ottl.StringGetter[any]](protocol)
			seedOpt := ottl.NewTestingOptional[ottl.IntGetter[any]](seed)

			exprFunc := communityID(sourceIP, sourcePort, destIP, destPort, protocolOpt, seedOpt)

			result, err := exprFunc(t.Context(), nil)
			if tt.errorExpected {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
