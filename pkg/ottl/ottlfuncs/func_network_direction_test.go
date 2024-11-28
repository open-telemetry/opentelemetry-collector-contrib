// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
)

func Test_NetworkDirection(t *testing.T) {
	testCases := []struct {
		sourceIP          string
		destinationIP     string
		internalNetworks  []string
		expectedDirection string
		expectedError     string
	}{
		// cidr range tests
		{"10.0.1.1", "192.168.1.2", []string{"10.0.0.0/8"}, "outbound", ""},
		{"192.168.1.2", "10.0.1.1", []string{"10.0.0.0/8"}, "inbound", ""},

		// private network tests
		{"192.168.1.1", "192.168.1.2", []string{"private"}, "internal", ""},
		{"10.0.1.1", "192.168.1.2", []string{"private"}, "internal", ""},
		{"192.168.1.1", "172.16.0.1", []string{"private"}, "internal", ""},
		{"192.168.1.1", "fd12:3456:789a:1::1", []string{"private"}, "internal", ""},

		// public network tests
		{"192.168.1.1", "192.168.1.2", []string{"public"}, "external", ""},
		{"10.0.1.1", "192.168.1.2", []string{"public"}, "external", ""},
		{"192.168.1.1", "172.16.0.1", []string{"public"}, "external", ""},
		{"192.168.1.1", "fd12:3456:789a:1::1", []string{"public"}, "external", ""},

		// unspecified tests
		{"0.0.0.0", "0.0.0.0", []string{"unspecified"}, "internal", ""},
		{"::", "::", []string{"unspecified"}, "internal", ""},

		// invalid inputs tests
		{"invalid", "192.168.1.2", []string{}, "", `source IP "invalid" is not a valid address`},
		{"192.168.1.1", "invalid", []string{}, "", `destination IP "invalid" is not a valid address`},
		{"192.168.1.1", "192.168.1.2", []string{"10.0.0.0/8", "invalid"}, "", `failed determining whether source IP is internal: invalid network definition for "invalid"`},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("test case #%d: %s -> %s", i, testCase.sourceIP, testCase.destinationIP), func(t *testing.T) {
			internalNetworksOptional := ottl.NewTestingOptional[[]string](testCase.internalNetworks)

			source := &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return testCase.sourceIP, nil
				},
			}

			destination := &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return testCase.destinationIP, nil
				},
			}
			exprFunc := networkDirection(source, destination, internalNetworksOptional)
			result, err := exprFunc(context.Background(), nil)
			if len(testCase.expectedError) > 0 {
				assert.Error(t, err)
				assert.Equal(t, testCase.expectedError, err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedDirection, result)
			}
		})
	}
}
