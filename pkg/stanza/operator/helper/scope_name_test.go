// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

const testScopeName = "my.logger"

func TestScopeNameParser(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name      string
		parser    *ScopeNameParser
		input     *entry.Entry
		expectErr bool
		expected  *entry.Entry
	}{
		{
			name: "root_string",
			parser: &ScopeNameParser{
				ParseFrom: entry.NewBodyField(),
			},
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = testScopeName
				e.ObservedTimestamp = now
				return e
			}(),
			expected: func() *entry.Entry {
				e := entry.New()
				e.Body = testScopeName
				e.ScopeName = testScopeName
				e.ObservedTimestamp = now
				return e
			}(),
		},
		{
			name: "nondestructive_error",
			parser: &ScopeNameParser{
				ParseFrom: entry.NewBodyField(),
			},
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]interface{}{"logger": testScopeName}
				e.ObservedTimestamp = now
				return e
			}(),
			expectErr: true,
			expected: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]interface{}{"logger": testScopeName}
				e.ObservedTimestamp = now
				return e
			}(),
		},
		{
			name: "nonroot_string",
			parser: &ScopeNameParser{
				ParseFrom: entry.NewBodyField("logger"),
			},
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]interface{}{"logger": testScopeName}
				e.ObservedTimestamp = now
				return e
			}(),
			expected: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]interface{}{"logger": testScopeName}
				e.ScopeName = testScopeName
				e.ObservedTimestamp = now
				return e
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.parser.Parse(tc.input)
			if tc.expectErr {
				require.Error(t, err)
			}
			if tc.expected != nil {
				require.Equal(t, tc.expected, tc.input)
			}
		})
	}
}

func TestUnmarshalScopeNameConfig(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: newHelpersConfig(),
		TestsFile:     filepath.Join(".", "testdata", "scope_name.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: "parse_from",
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Scope = NewScopeNameParser()
					c.Scope.ParseFrom = entry.NewBodyField("from")
					return c
				}(),
			},
		},
	}.Run(t)
}
