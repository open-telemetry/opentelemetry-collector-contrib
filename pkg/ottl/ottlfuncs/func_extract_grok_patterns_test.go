// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_extractGrokPatterns_patterns(t *testing.T) {
	tests := []struct {
		name              string
		targetString      string
		pattern           string
		namedCapturesOnly bool
		want              func(pcommon.Map)
		definitions       []string
	}{
		{
			name:              "regex - extract patterns",
			targetString:      `a=b c=d`,
			pattern:           `^a=(?P<a>\w+)\s+c=(?P<c>\w+)$`,
			namedCapturesOnly: false,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("a", "b")
				expectedMap.PutStr("c", "d")
			},
			definitions: nil,
		},
		{
			name:              "regex - no pattern found",
			targetString:      `a=b c=d`,
			namedCapturesOnly: false,
			pattern:           `^a=(?P<a>\w+)$`,
			want:              func(_ pcommon.Map) {},
		},
		{
			name:              "grok - URI default pattern",
			targetString:      `http://user:password@example.com:80/path?query=string`,
			pattern:           "%{URI}",
			namedCapturesOnly: false,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("URIPROTO", "http")
				expectedMap.PutStr("USER", "user")
				expectedMap.PutStr("URIHOST", "example.com:80")
				expectedMap.PutStr("IPORHOST", "example.com")
				expectedMap.PutStr("POSINT", "80")
				expectedMap.PutStr("URIPATH", "/path")
				expectedMap.PutStr("URIQUERY", "query=string")
			},
			definitions: nil,
		},
		{
			name:              "grok - URI AWS pattern with captures",
			targetString:      `http://user:password@example.com:80/path?query=string`,
			pattern:           "%{ELB_URI}",
			namedCapturesOnly: true,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("url.scheme", "http")
				expectedMap.PutStr("url.username", "user")
				expectedMap.PutStr("url.domain", "example.com")
				expectedMap.PutInt("url.port", 80)
				expectedMap.PutStr("url.path", "/path")
				expectedMap.PutStr("url.query", "query=string")
			},
			definitions: nil,
		},
		{
			name:              "grok - POSTGRES log sample",
			targetString:      `2024-06-18 12:34:56 UTC johndoe 12345 67890`,
			pattern:           "%{DATESTAMP:timestamp} %{TZ:event.timezone} %{DATA:user.name} %{GREEDYDATA:postgresql.log.connection_id} %{POSINT:process.pid:int}",
			namedCapturesOnly: true,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("timestamp", "24-06-18 12:34:56")
				expectedMap.PutStr("event.timezone", "UTC")
				expectedMap.PutStr("user.name", "johndoe")
				expectedMap.PutStr("postgresql.log.connection_id", "12345")
				expectedMap.PutInt("process.pid", 67890)
			},
			definitions: nil,
		},
		{
			name:              "grok - custom patterns",
			targetString:      `2024-06-18 12:34:56 otel`,
			pattern:           "%{MYPATTERN}",
			namedCapturesOnly: true,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("timestamp", "24-06-18 12:34:56")
			},
			definitions: []string{
				`MYPATTERN=%{MYDATEPATTERN:timestamp} otel`,
				`MYDATEPATTERN=%{DATE}[- ]%{TIME}`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nco := ottl.NewTestingOptional(tt.namedCapturesOnly)
			target := &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tt.targetString, nil
				},
			}

			patternDefinitionOptional := ottl.NewTestingOptional[[]string](tt.definitions)
			exprFunc, err := extractGrokPatterns(target, tt.pattern, nco, patternDefinitionOptional)
			assert.NoError(t, err)

			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)

			resultMap, ok := result.(pcommon.Map)
			require.True(t, ok)

			expected := pcommon.NewMap()
			tt.want(expected)

			for k := range expected.All() {
				ev, _ := expected.Get(k)
				av, _ := resultMap.Get(k)
				assert.Equal(t, ev, av)
			}
		})
	}
}

func Test_extractGrokPatterns_validation(t *testing.T) {
	tests := []struct {
		name                 string
		target               ottl.StringGetter[any]
		pattern              string
		namedCapturesOnly    bool
		definitions          []string
		expectedFactoryError bool
		expectedError        bool
	}{
		{
			name: "bad regex",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "foobar", nil
				},
			},
			pattern:              "(",
			namedCapturesOnly:    false,
			expectedFactoryError: true,
		},
		{
			name: "no named capture group",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "foobar", nil
				},
			},
			pattern:           "(.*)",
			namedCapturesOnly: false,
			expectedError:     false,
		},
		{
			name: "custom pattern name invalid",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "http://user:password@example.com:80/path?query=string", nil
				},
			},
			pattern: "%{URI}",
			definitions: []string{
				"PAT:TERN=invalid",
			},
			expectedFactoryError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nco := ottl.NewTestingOptional(tt.namedCapturesOnly)
			patternDefinitionOptional := ottl.NewTestingOptional[[]string](tt.definitions)
			exprFunc, err := extractGrokPatterns[any](tt.target, tt.pattern, nco, patternDefinitionOptional)
			if tt.expectedFactoryError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			_, err = exprFunc(nil, nil)
			assert.Equal(t, tt.expectedError, err != nil)
		})
	}
}

func Test_extractGrokPatterns_bad_input(t *testing.T) {
	tests := []struct {
		name    string
		target  ottl.StringGetter[any]
		pattern string
	}{
		{
			name: "regex - target is non-string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 123, nil
				},
			},
			pattern: "(?P<line>.*)",
		},
		{
			name: "regex - target is nil",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
			},
			pattern: "(?P<line>.*)",
		},
		{
			name: "target is nil",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
			},
			pattern: "%{URI}",
		},
		{
			name: "target is non-string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 123, nil
				},
			},
			pattern: "%{URI}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nco := ottl.NewTestingOptional(false)
			patternDefinitionOptional := ottl.NewTestingOptional[[]string](nil)

			exprFunc, err := extractGrokPatterns[any](tt.target, tt.pattern, nco, patternDefinitionOptional)
			assert.NoError(t, err)

			result, err := exprFunc(nil, nil)
			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}
