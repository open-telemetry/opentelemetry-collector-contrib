// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cases := []struct {
		name        string
		regex       string
		values      []string
		expectedErr string
		expectItems []*item
		numOpts     int
	}{
		{
			name:   "SingleCapture",
			regex:  `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
			values: []string{"err.2023020611.log", "err.2023020612.log"},
			expectItems: []*item{
				{
					value:    "err.2023020611.log",
					captures: map[string]string{"value": "2023020611"},
				},
				{
					value:    "err.2023020612.log",
					captures: map[string]string{"value": "2023020612"},
				},
			},
		},
		{
			name:  "MultipleCapture",
			regex: `foo\.(?P<alpha>[a-zA-Z])\.(?P<number>\d+)\.(?P<time>\d{10})\.log`,
			values: []string{
				"foo.b.1.2023020601.log",
				"foo.a.2.2023020602.log",
			},
			expectItems: []*item{
				{
					value: "foo.b.1.2023020601.log",
					captures: map[string]string{
						"alpha":  "b",
						"number": "1",
						"time":   "2023020601",
					},
				},
				{
					value: "foo.a.2.2023020602.log",
					captures: map[string]string{
						"alpha":  "a",
						"number": "2",
						"time":   "2023020602",
					},
				},
			},
			numOpts: 2,
		},
		{
			name:        "OneInvalid",
			regex:       `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
			values:      []string{"err.2023020611.log", "foo.2023020612.log"},
			expectedErr: "'foo.2023020612.log' does not match regex",
			expectItems: []*item{
				{
					value:    "err.2023020611.log",
					captures: map[string]string{"value": "2023020611"},
				},
			},
			numOpts: 3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testOpt := &testOpt{}
			opts := make([]Option, 0, tc.numOpts)
			for i := 0; i < tc.numOpts; i++ {
				opts = append(opts, testOpt)
			}
			f, err := New(tc.values, regexp.MustCompile(tc.regex), opts...)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.numOpts, len(f.opts))
			assert.Equal(t, tc.expectItems, f.items)
			values := f.Values()
			for i := range values {
				assert.Equal(t, tc.expectItems[i].value, values[i])
			}
		})
	}
}

type testOpt struct{}

func (o *testOpt) apply(_ *Filter) error {
	return nil
}
