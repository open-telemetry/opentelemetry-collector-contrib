// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc     string
		cfg      Config
		expected error
	}{
		{
			desc:     "no configuration",
			cfg:      Config{},
			expected: errHTTPEndpointRequired,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			res := tC.cfg.Validate()
			assert.Equal(t, tC.expected, res)
		})
	}
}
