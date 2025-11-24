// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ProfileIDArguments[K any] struct {
	Target ottl.ByteSliceLikeGetter[K]
}

func NewProfileIDFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ProfileID", &ProfileIDArguments[K]{}, createProfileIDFunction[K])
}

func createProfileIDFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ProfileIDArguments[K])

	if !ok {
		return nil, errors.New("ProfileIDFactory args must be of type *ProfileIDArguments[K]")
	}

	return profileID[K](args.Target), nil
}

func profileID[K any](target ottl.ByteSliceLikeGetter[K]) ottl.ExprFunc[K] {
	return newIDExprFunc[K, pprofile.ProfileID](target)
}
