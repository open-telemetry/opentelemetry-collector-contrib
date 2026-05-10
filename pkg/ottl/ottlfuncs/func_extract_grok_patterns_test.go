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
			pattern:           `%{ELB_URI}`,
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
			pattern:           `%{DATESTAMP:timestamp} %{TZ:event.timezone} %{DATA:user.name} %{GREEDYDATA:postgresql.log.connection_id} %{POSINT:process.pid:int}`,
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
			name:              "grok - long capture keeps int64 timestamp",
			targetString:      `1686838825123 INFO`,
			pattern:           `%{POSINT:timestamp:long} %{WORD:level}`,
			namedCapturesOnly: true,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutInt("timestamp", 1686838825123)
				expectedMap.PutStr("level", "INFO")
			},
			definitions: nil,
		},
		{
			name:              "grok - custom patterns",
			targetString:      `2024-06-18 12:34:56 otel`,
			pattern:           `%{MYPATTERN}`,
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
				Getter: func(context.Context, any) (any, error) {
					return tt.targetString, nil
				},
			}

			patternDefinitionOptional := ottl.NewTestingOptional[[]string](tt.definitions)
			pattern := &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.pattern, nil
				},
			}
			exprFunc, err := extractGrokPatterns(target, pattern, nco, patternDefinitionOptional)
			require.NoError(t, err)

			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)

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

func Test_putGrokValueKeepsInt64Captures(t *testing.T) {
	result := pcommon.NewMap()

	putGrokValue(result, "timestamp", int64(1778281254690))

	got, ok := result.Get("timestamp")
	require.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeInt, got.Type())
	assert.Equal(t, int64(1778281254690), got.Int())
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
				Getter: func(context.Context, any) (any, error) {
					return "foobar", nil
				},
			},
			pattern:           "(",
			namedCapturesOnly: false,
			expectedError:     true,
		},
		{
			name: "no named capture group",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
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
				Getter: func(context.Context, any) (any, error) {
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
			pattern := &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.pattern, nil
				},
			}
			exprFunc, err := extractGrokPatterns[any](tt.target, pattern, nco, patternDefinitionOptional)
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
				Getter: func(context.Context, any) (any, error) {
					return 123, nil
				},
			},
			pattern: "(?P<line>.*)",
		},
		{
			name: "regex - target is nil",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return nil, nil
				},
			},
			pattern: "(?P<line>.*)",
		},
		{
			name: "target is nil",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return nil, nil
				},
			},
			pattern: "%{URI}",
		},
		{
			name: "target is non-string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
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

			pattern := &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.pattern, nil
				},
			}
			exprFunc, err := extractGrokPatterns[any](tt.target, pattern, nco, patternDefinitionOptional)
			require.NoError(t, err)

			result, err := exprFunc(nil, nil)
			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

func Test_newGrokLiteralPrefilter(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		wantNil   bool
		match     string
		mismatch  string
		mismatch2 string
	}{
		{
			name:     "required literals around grok placeholders",
			pattern:  `HealthProducer - sending health message: %{word:status}`,
			match:    `INFO HealthProducer - sending health message: green`,
			mismatch: `INFO unrelated message`,
		},
		{
			name:      "keeps literals around nested alternation",
			pattern:   `prefix (foo|bar) required suffix %{word:value}`,
			match:     `prefix foo required suffix ok`,
			mismatch:  `prefix foo missing suffix ok`,
			mismatch2: `required suffix only`,
		},
		{
			name:     "keeps literals inside mandatory groups",
			pattern:  `prefix (?:required marker) suffix %{word:value}`,
			match:    `prefix required marker suffix ok`,
			mismatch: `prefix other marker suffix ok`,
		},
		{
			name:     "does not require literals inside optional groups",
			pattern:  `prefix (?:optional marker)? suffix %{word:value}`,
			match:    `prefix suffix ok`,
			mismatch: `prefix optional marker ok`,
		},
		{
			name:    "skips unsafe top-level alternation",
			pattern: `foo|bar`,
			wantNil: true,
		},
		{
			name:    "skips case-insensitive patterns",
			pattern: `(?i)health.*check`,
			wantNil: true,
		},
		{
			name:    "skips scoped case-insensitive patterns",
			pattern: `(?i:health).*check`,
			wantNil: true,
		},
		{
			name:    "skips scoped mixed-flag case-insensitive patterns",
			pattern: `(?mi:health).*check`,
			wantNil: true,
		},
		{
			name:    "skips fallback combined case-insensitive patterns",
			pattern: `(?im)health(?<=done)%{WORD:value}`,
			wantNil: true,
		},
		{
			name:    "skips fallback enabled case-insensitive reset patterns",
			pattern: `(?i-m)health(?<=done)%{WORD:value}`,
			wantNil: true,
		},
		{
			name:     "skips fallback brace quantifier bytes",
			pattern:  `foo{1,2}bar(?<=done)%{WORD:value}`,
			match:    `foobar done ok`,
			mismatch: `foo done ok`,
		},
		{
			name:     "treats zero minimum fallback brace quantifier as optional",
			pattern:  `foo{0,2}bar(?<=done)%{WORD:value}`,
			match:    `fobar done ok`,
			mismatch: `f done ok`,
		},
		{
			name:    "skips literals inside character classes in lookaheads",
			pattern: `(?=[^)]foo)%{GREEDYDATA:value}`,
			wantNil: true,
		},
		{
			name:     "keeps escaped punctuation literals",
			pattern:  `\\[tenantId: %{notSpace:tenant_id}\\] completed`,
			match:    `[tenantId: tenant-a] completed`,
			mismatch: `[tenantId: tenant-a] started`,
		},
		{
			name:     "keeps utf8 required literals",
			pattern:  `你好 %{word:value}(?= 完成)`,
			match:    `INFO 你好 green 完成`,
			mismatch: `INFO hello green done`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefilter := newGrokLiteralPrefilter(tt.pattern)
			if tt.wantNil {
				require.Nil(t, prefilter)
				return
			}
			require.NotNil(t, prefilter)
			require.True(t, prefilter(tt.match))
			require.False(t, prefilter(tt.mismatch))
			if tt.mismatch2 != "" {
				require.False(t, prefilter(tt.mismatch2))
			}
		})
	}
}

func Test_newGrokLiteralPrefilterUsesPatternDefinitions(t *testing.T) {
	prefilter := newGrokLiteralPrefilter(
		`%{LOGLINE}`,
		`LOGLINE=%{DATESTAMP:timestamp} fixed marker %{WORD:value}`,
	)

	require.NotNil(t, prefilter)
	require.True(t, prefilter(`2024-06-18 12:34:56 fixed marker ok`))
	require.False(t, prefilter(`2024-06-18 12:34:56 other marker ok`))
}

func Test_extractRequiredGrokLiteralsFallbackKeepsMandatoryNonCapturingGroup(t *testing.T) {
	literals := extractRequiredGrokLiterals(`(?:required marker)(?= suffix)`)

	require.Contains(t, literals, "required marker")
}

func Test_extractRequiredGrokLiteralsFallbackSkipsOptionalNonCapturingGroup(t *testing.T) {
	literals := extractRequiredGrokLiterals(`(?:optional marker)?(?= suffix)`)

	require.NotContains(t, literals, "optional marker")
}

func Test_extractRequiredGrokLiteralsFallbackSkipsOptionalQuantifiedByte(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		input   string
	}{
		{
			name:    "question mark",
			pattern: `foo?bar(?<=unsupported)`,
			input:   "fobar unsupported",
		},
		{
			name:    "asterisk",
			pattern: `foo*bar(?<=unsupported)`,
			input:   "fbar unsupported",
		},
		{
			name:    "plus",
			pattern: `foo+bar(?<=unsupported)`,
			input:   "foobar unsupported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			literals := extractRequiredGrokLiterals(tt.pattern)
			require.Contains(t, literals, "bar")
			if tt.name == "plus" {
				require.Contains(t, literals, "foo")
			} else {
				require.NotContains(t, literals, "foo")
			}

			prefilter := newGrokLiteralPrefilter(tt.pattern)
			require.NotNil(t, prefilter)
			require.True(t, prefilter(tt.input))
		})
	}
}
