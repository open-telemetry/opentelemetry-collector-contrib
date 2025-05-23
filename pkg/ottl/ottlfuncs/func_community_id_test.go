// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestCommunityID(t *testing.T) {
	tests := []struct {
		name           string
		sourceIP       string
		destIP         string
		sourcePort     string
		destPort       string
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
			sourcePort:    "12345",
			destPort:      "80",
			protocol:      "TCP",
			seed:          0,
			expected:      "1:MRwiG16AwBQWiBNe8QM/CDKBpC0=",
			errorExpected: false,
		},
		{
			name:          "UDP IPv4 Example",
			sourceIP:      "192.168.1.1",
			destIP:        "10.10.10.10",
			sourcePort:    "53",
			destPort:      "53000",
			protocol:      "UDP",
			seed:          0,
			expected:      "1:P7Q5E9XRN1k1cfs7EIWXO4owerM=",
			errorExpected: false,
		},
		{
			name:          "TCP IPv6 Example",
			sourceIP:      "2001:db8::1",
			destIP:        "2001:db8::2",
			sourcePort:    "8080",
			destPort:      "443",
			protocol:      "TCP",
			seed:          0,
			expected:      "1:3kZ62CG92qhcjtwkd1ZCnGDhKew=",
			errorExpected: false,
		},
		{
			name:          "Custom Seed Example",
			sourceIP:      "192.168.1.1",
			destIP:        "192.168.1.2",
			sourcePort:    "12345",
			destPort:      "80",
			protocol:      "TCP",
			seed:          1234,
			expected:      "1:YZFTN4zsIh19iLm0qfioslw/NZY=",
			errorExpected: false,
		},
		{
			name:          "Normalized Direction Example",
			sourceIP:      "10.0.0.2", // Higher IP should be normalized
			destIP:        "10.0.0.1",
			sourcePort:    "80",
			destPort:      "12345",
			protocol:      "TCP",
			seed:          0,
			expected:      "1:N1t5gWxL3d82YyCQfwN+7iVfX0M=",
			errorExpected: false,
		},
		{
			name:           "Invalid Source IP",
			sourceIP:       "invalid-ip",
			destIP:         "10.0.0.1",
			sourcePort:     "80",
			destPort:       "12345",
			protocol:       "TCP",
			seed:           0,
			errorExpected:  true,
			expectedErrMsg: "invalid source IP",
		},
		{
			name:           "Invalid Protocol",
			sourceIP:       "10.0.0.1",
			destIP:         "10.0.0.2",
			sourcePort:     "80",
			destPort:       "12345",
			protocol:       "UNKNOWN",
			seed:           0,
			errorExpected:  true,
			expectedErrMsg: "unsupported protocol",
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
					port, _ := strconv.ParseInt(tt.sourcePort, 10, 64)
					return port, nil
				},
			}
			destPort := ottl.StandardIntGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					port, _ := strconv.ParseInt(tt.destPort, 10, 64)
					return port, nil
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

			exprFunc, err := CommunityIDHash(sourceIP, sourcePort, destIP, destPort, protocolOpt, seedOpt)
			require.NoError(t, err)

			result, err := exprFunc(context.Background(), nil)
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
