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

package scope

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

const testScopeName = "my.logger"

func TestScopeNameParser(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name      string
		config    *ParserConfig
		input     *entry.Entry
		expectErr bool
		expected  *entry.Entry
	}{
		{
			name: "root_string",
			config: func() *ParserConfig {
				cfg := NewParserConfig("test")
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
				e.ScopeName = testScopeName
				e.ObservedTimestamp = now
				return e
			}(),
		},
		{
			name: "root_string_preserve",
			config: func() *ParserConfig {
				cfg := NewParserConfig("test")
				cfg.ParseFrom = entry.NewBodyField()
				preserve := entry.NewBodyField()
				cfg.PreserveTo = &preserve
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
			config: func() *ParserConfig {
				cfg := NewParserConfig("test")
				cfg.ParseFrom = entry.NewBodyField()
				return cfg
			}(),
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
			config: func() *ParserConfig {
				cfg := NewParserConfig("test")
				cfg.ParseFrom = entry.NewBodyField("logger")
				return cfg
			}(),
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]interface{}{"logger": testScopeName}
				e.ObservedTimestamp = now
				return e
			}(),
			expected: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]interface{}{}
				e.ScopeName = testScopeName
				e.ObservedTimestamp = now
				return e
			}(),
		},
		{
			name: "nonroot_string_preserve",
			config: func() *ParserConfig {
				cfg := NewParserConfig("test")
				cfg.ParseFrom = entry.NewBodyField("logger")
				preserve := entry.NewBodyField("somewhere")
				cfg.PreserveTo = &preserve
				return cfg
			}(),
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]interface{}{"logger": testScopeName}
				e.ObservedTimestamp = now
				return e
			}(),
			expected: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]interface{}{"somewhere": testScopeName}
				e.ScopeName = testScopeName
				e.ObservedTimestamp = now
				return e
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser, err := tc.config.Build(testutil.Logger(t))
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
