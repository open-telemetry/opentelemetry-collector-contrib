// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeduplicateErrors(t *testing.T) {
	testCases := []struct {
		name     string
		errs     []error
		expected []error
	}{
		{
			name:     "nil is returned as nil",
			errs:     nil,
			expected: nil,
		},
		{
			name: "single error is returned as-is",
			errs: []error{
				errors.New("Single error"),
			},
			expected: []error{
				errors.New("Single error"),
			},
		},
		{
			name: "duplicates are removed",
			errs: []error{
				errors.New("failed sending data: 502 Bad Gateway"),
				errors.New("failed sending data: 400 Bad Request"),
				errors.New("failed sending data: 502 Bad Gateway"),
				errors.New("failed sending data: 400 Bad Request"),
				errors.New("failed sending data: 400 Bad Request"),
				errors.New("failed sending data: 400 Bad Request"),
				errors.New("failed sending data: 504 Gateway Timeout"),
				errors.New("failed sending data: 502 Bad Gateway"),
			},
			expected: []error{
				errors.New("failed sending data: 502 Bad Gateway (x3)"),
				errors.New("failed sending data: 400 Bad Request (x4)"),
				errors.New("failed sending data: 504 Gateway Timeout"),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, deduplicateErrors(testCase.errs))
		})
	}
}
