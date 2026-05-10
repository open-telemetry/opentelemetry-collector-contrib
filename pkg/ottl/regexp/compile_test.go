// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package regexp

import (
	"errors"
	stdregexp "regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	re2 "github.com/wasilibs/go-re2"
)

func TestCompileSelectsEngineByPatternLength(t *testing.T) {
	shortPattern := strings.Repeat("a", patternLengthThreshold-1)
	shortMatcher, err := Compile(shortPattern)
	require.NoError(t, err)
	require.IsType(t, &stdregexp.Regexp{}, shortMatcher)

	longPattern := strings.Repeat("a", patternLengthThreshold)
	longMatcher, err := Compile(longPattern)
	require.NoError(t, err)
	require.IsType(t, &re2.Regexp{}, longMatcher)
}

func TestCompileReturnsEngineErrors(t *testing.T) {
	shortMatcher, err := Compile("(")
	require.Error(t, err)
	require.Nil(t, shortMatcher)

	longMatcher, err := Compile(strings.Repeat("a", patternLengthThreshold) + "(")
	require.Error(t, err)
	require.Nil(t, longMatcher)
}

func TestCompileFallsBackToStdlibWhenRE2CompileFails(t *testing.T) {
	originalCompileRE2 := compileRE2
	t.Cleanup(func() {
		compileRE2 = originalCompileRE2
	})
	compileRE2 = func(string) (*re2.Regexp, error) {
		return nil, errors.New("forced re2 compile error")
	}

	matcher, err := Compile(strings.Repeat("a", patternLengthThreshold))
	require.NoError(t, err)
	require.IsType(t, &stdregexp.Regexp{}, matcher)
}

func TestCompileUsesLeftmostFirstSemantics(t *testing.T) {
	pattern := strings.Repeat("(?:x)?", patternLengthThreshold/5) + "(a|aa)"

	matcher, err := Compile(pattern)
	require.NoError(t, err)
	require.IsType(t, &re2.Regexp{}, matcher)
	require.Equal(t, []string{"a", "a"}, matcher.FindStringSubmatch("aa"))
}

func TestMustCompilePanicsOnEngineErrors(t *testing.T) {
	require.Panics(t, func() {
		MustCompile("(")
	})
	require.Panics(t, func() {
		MustCompile(strings.Repeat("a", patternLengthThreshold) + "(")
	})
}
