// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ProfileIDArguments[K any] struct {
	Bytes []byte
}

func NewProfileIDFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ProfileID", &ProfileIDArguments[K]{}, createProfileIDFunction[K])
}

func createProfileIDFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ProfileIDArguments[K])

	if !ok {
		return nil, errors.New("ProfileIDFactory args must be of type *ProfileIDArguments[K]")
	}

	return profileID[K](args.Bytes)
}

var emptyProfileID = pprofile.NewProfileIDEmpty()

func profileID[K any](bytes []byte) (ottl.ExprFunc[K], error) {
	if len(bytes) != len(emptyProfileID) {
		return nil, fmt.Errorf("profile ids must be %d bytes", len(emptyProfileID))
	}
	if pprofile.ProfileID(bytes) == emptyProfileID {
		return nil, fmt.Errorf("profile ids cannot be all zeroes")
	}

	id := pprofile.ProfileID{}
	copy(id[:], bytes)

	return func(context.Context, K) (any, error) {
		return id, nil
	}, nil
}
