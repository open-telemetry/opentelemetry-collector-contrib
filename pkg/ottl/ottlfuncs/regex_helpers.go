// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"fmt"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const invalidRegexErrMsg = "the regex pattern supplied to %s '%q' is not a valid pattern: %w"

// compileRegexPattern attempts to get a literal pattern and compile it at creation time.
// Returns the compiled pattern if successful, nil otherwise.
func compileRegexPattern[K any](pattern ottl.StringGetter[K], functionName string) (*regexp.Regexp, error) {
	literalPattern, ok := ottl.GetLiteralValue(pattern)
	if !ok {
		return nil, nil
	}
	compiled, err := regexp.Compile(literalPattern)
	if err != nil {
		return nil, fmt.Errorf(invalidRegexErrMsg, functionName, literalPattern, err)
	}
	return compiled, nil
}

// getCompiledRegex returns a compiled regex pattern. If a pre-compiled pattern exists, it returns that.
// Otherwise, it gets the pattern value and compiles it at runtime.
func getCompiledRegex[K any](ctx context.Context, tCtx K, pattern ottl.StringGetter[K], precompiled *regexp.Regexp, functionName string) (*regexp.Regexp, error) {
	if precompiled != nil {
		return precompiled, nil
	}
	patternVal, err := pattern.Get(ctx, tCtx)
	if err != nil {
		return nil, err
	}
	compiled, err := regexp.Compile(patternVal)
	if err != nil {
		return nil, fmt.Errorf(invalidRegexErrMsg, functionName, patternVal, err)
	}
	return compiled, nil
}
