// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewItem(t *testing.T) {
	cases := []struct {
		name        string
		regex       string
		value       string
		expectedErr string
		expect      *item
	}{
		{
			name:  "SingleCapture",
			regex: `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
			value: "err.2023020611.log",
			expect: &item{
				value:    "err.2023020611.log",
				captures: map[string]string{"value": "2023020611"},
			},
		},
		{
			name:  "MultipleCapture",
			regex: `foo\.(?P<alpha>[a-zA-Z])\.(?P<number>\d+)\.(?P<time>\d{10})\.log`,
			value: "foo.b.1.2023020601.log",
			expect: &item{
				value: "foo.b.1.2023020601.log",
				captures: map[string]string{
					"alpha":  "b",
					"number": "1",
					"time":   "2023020601",
				},
			},
		},
		{
			name:        "Invalid",
			regex:       `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
			value:       "foo.2023020612.log",
			expectedErr: "'foo.2023020612.log' does not match regex",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			it, err := newItem(tc.value, regexp.MustCompile(tc.regex))
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expect, it)
		})
	}
}

func TestFilter(t *testing.T) {
	cases := []struct {
		name        string
		regex       string
		values      []string
		expectedErr string
		expect      []string
		numOpts     int
	}{
		{
			name:   "SingleCapture",
			regex:  `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
			values: []string{"err.2023020611.log", "err.2023020612.log"},
			expect: []string{"err.2023020611.log", "err.2023020612.log"},
		},
		{
			name:    "MultipleCapture",
			regex:   `foo\.(?P<alpha>[a-zA-Z])\.(?P<number>\d+)\.(?P<time>\d{10})\.log`,
			values:  []string{"foo.b.1.2023020601.log", "foo.a.2.2023020602.log", "foo.a.2.2023020603.log"},
			expect:  []string{"foo.a.2.2023020603.log"},
			numOpts: 2,
		},
		{
			name:        "OneInvalid",
			regex:       `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
			values:      []string{"err.2023020611.log", "foo.2023020612.log"},
			expectedErr: "'foo.2023020612.log' does not match regex",
			expect:      []string{"err.2023020611.log"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := make([]Option, 0, tc.numOpts)
			for i := 0; i < tc.numOpts; i++ {
				opts = append(opts, &removeFirst{})
			}
			result, err := Filter(tc.values, regexp.MustCompile(tc.regex), opts...)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expect, result)
		})
	}
}

type removeFirst struct{}

func (o *removeFirst) apply(items []*item) ([]*item, error) {
	if len(items) == 0 {
		return items, nil
	}
	return items[1:], nil
}
