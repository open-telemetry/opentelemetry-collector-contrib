// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/canva/otel-platform/opentelemetry-collector/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IsMatchVersionArguments[K any] struct {
	Target     ottl.StringGetter[K]
	Constraint string
}

func NewIsMatchVersionFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsMatchVersion", &IsMatchVersionArguments[K]{}, createIsMatchVersionFunction[K])
}

func createIsMatchVersionFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsMatchVersionArguments[K])

	if !ok {
		return nil, fmt.Errorf("IsMatchVersionFactory args must be of type *IsMatchVersionArguments[K]")
	}

	return isMatchVersion(args.Target, args.Constraint)
}

func isMatchVersion[K any](target ottl.StringGetter[K], constraint string) (ottl.ExprFunc[K], error) {
	semverconstraint, err := semver.NewConstraint(constraint)
	if err != nil {
		return nil, fmt.Errorf("the constrain supplied to IsMatchVersion is not a valid: %w", err)
	}
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)

		if err != nil {
			return false, err
		}

		version, err := semver.NewVersion(val)

		if err != nil {
			return false, fmt.Errorf("failed to parse semver version from: %s, err: %w", val, err)
		}

		return semverconstraint.Check(version), nil
	}, nil
}
