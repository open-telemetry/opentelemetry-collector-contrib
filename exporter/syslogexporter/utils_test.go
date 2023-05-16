// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

func TestErrorString(t *testing.T) {
	testCases := []struct {
		name     string
		errs     []error
		expected []string
	}{
		{
			name: "duplicates are removed",
			errs: []error{
				errors.New("failed sending data: 502 Bad Gateway"),
				errors.New("dial tcp 127.0.0.1:514: connect: connection refused"),
			},
			expected: []string{"failed sending data: 502 Bad Gateway",
				"dial tcp 127.0.0.1:514: connect: connection refused"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, errorListToStringSlice(testCase.errs))
		})
	}
}
