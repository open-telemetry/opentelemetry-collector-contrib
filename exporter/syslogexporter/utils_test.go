// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter

import (
	"errors"
	"fmt"
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
				errors.New("dial tcp 127.0.0.1:514: connect: connection refused"),
				errors.New("failed sending data: 502 Bad Gateway"),
				errors.New("dial tcp 127.0.0.1:514: connect: connection refused"),
				errors.New("dial tcp 127.0.0.1:514: connect: connection refused"),
				errors.New("dial tcp 127.0.0.1:514: connect: connection refused"),
				errors.New("failed sending data: 504 Gateway Timeout"),
				errors.New("failed sending data: 502 Bad Gateway"),
			},
			expected: []error{
				fmt.Errorf("%w (x3)", errors.New("failed sending data: 502 Bad Gateway")),
				fmt.Errorf("%w (x4)", errors.New("dial tcp 127.0.0.1:514: connect: connection refused")),
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
