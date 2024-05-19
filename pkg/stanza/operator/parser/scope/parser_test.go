// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scope

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

const testScopeName = "my.logger"

func TestScopeNameParser(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name      string
		config    *Config
		input     *entry.Entry
		expectErr bool
		expected  *entry.Entry
	}{
		{
			name: "root_string",
			config: func() *Config {
				cfg := NewConfigWithID("test")
				cfg.ParseFrom = entry.NewBodyField()
				return cfg
			}(),
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
			config: func() *Config {
				cfg := NewConfigWithID("test")
				cfg.ParseFrom = entry.NewBodyField()
				return cfg
			}(),
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]any{"logger": testScopeName}
				e.ObservedTimestamp = now
				return e
			}(),
			expectErr: true,
			expected: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]any{"logger": testScopeName}
				e.ObservedTimestamp = now
				return e
			}(),
		},
		{
			name: "nonroot_string",
			config: func() *Config {
				cfg := NewConfigWithID("test")
				cfg.ParseFrom = entry.NewBodyField("logger")
				return cfg
			}(),
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]any{"logger": testScopeName}
				e.ObservedTimestamp = now
				return e
			}(),
			expected: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]any{"logger": testScopeName}
				e.ScopeName = testScopeName
				e.ObservedTimestamp = now
				return e
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			set := componenttest.NewNopTelemetrySettings()
			parser, err := tc.config.Build(set)
			require.NoError(t, err)

			err = parser.Process(context.Background(), tc.input)
			if tc.expectErr {
				require.Error(t, err)
			}
			if tc.expected != nil {
				require.Equal(t, tc.expected, tc.input)
			}
		})
	}
}
