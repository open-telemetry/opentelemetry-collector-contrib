// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var (
	ErrRenameKeyIsMissing            = errors.New("key is missing")
	ErrRenameInvalidConflictStrategy = errors.New("invalid conflict strategy")
	ErrRenameKeyAlreadyExists        = errors.New("key already exists")
)

const (
	renameConflictFail   = "fail"
	renameConflictInsert = "insert"
	renameConflictUpsert = "upsert"
)

// rename(map, field, target_field, [Optional] ignore_missing = true, [Optional] conflict_strategy = replace)
type RenameArguments[K any] struct {
	Map              ottl.PMapGetter[K]
	Field            string
	TargetField      string
	IgnoreMissing    ottl.Optional[bool]
	ConflictStrategy ottl.Optional[string]
}

func NewRenameFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("rename", &RenameArguments[K]{}, createRenameFunction[K])
}

func createRenameFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*RenameArguments[K])

	if !ok {
		return nil, fmt.Errorf("RenameFactory args must be of type *RenameArguments[K]")
	}

	return rename(args.Map, args.Field, args.TargetField, args.IgnoreMissing, args.ConflictStrategy)
}

func rename[K any](mg ottl.PMapGetter[K], f string, tf string, im ottl.Optional[bool], cs ottl.Optional[string]) (ottl.ExprFunc[K], error) {
	conflictStrategy := renameConflictUpsert
	if !cs.IsEmpty() {
		conflictStrategy = cs.Get()
	}

	if conflictStrategy != renameConflictUpsert &&
		conflictStrategy != renameConflictFail &&
		conflictStrategy != renameConflictInsert {
		return nil, fmt.Errorf("%v %w, must be %q, %q or %q", conflictStrategy, ErrRenameInvalidConflictStrategy, renameConflictUpsert, renameConflictFail, renameConflictInsert)
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		m, err := mg.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		// Do not ignore missing field by default
		ignoreMissing := true
		if !im.IsEmpty() {
			ignoreMissing = im.Get()
		}

		val, exists := m.Get(f)

		// Apply ignore_missing to the source
		if !exists {
			if ignoreMissing {
				// If ignore missing return
				return nil, nil
			}
			return nil, fmt.Errorf("%v %w, while ignore_missing is false", f, ErrRenameKeyIsMissing)
		}

		// Apply conflict_strategy to the target
		_, oldExists := m.Get(tf)

		switch conflictStrategy {
		case renameConflictUpsert:
			// Overwrite if present or create when missing
		case renameConflictFail:
			// Fail if target field present
			if oldExists {
				return nil, ErrRenameKeyAlreadyExists
			}
		case renameConflictInsert:
			// Noop if target field present
			if oldExists {
				return nil, nil
			}
		}

		// If field and targetField are the same
		if f == tf {
			return nil, nil
		}

		// Copy field value to targetField
		val.CopyTo(m.PutEmpty(tf))

		// Remove field from map
		m.Remove(f)
		return nil, nil
	}, nil
}
