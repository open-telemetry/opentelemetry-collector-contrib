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
	renameConflictInsert = "insert"
	renameConflictUpsert = "upsert"
	renameConflictFail   = "fail"
)

// rename(target, source_map, source_key, [Optional] ignore_missing = true, [Optional] conflict_strategy = upsert)
type RenameArguments[K any] struct {
	Target    ottl.GetSetter[K]
	SourceMap ottl.PMapGetter[K]
	SourceKey string

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

	return rename(args.Target, args.SourceMap, args.SourceKey, args.IgnoreMissing, args.ConflictStrategy)
}

func rename[K any](target ottl.GetSetter[K], sm ottl.PMapGetter[K], sourceKey string,
	im ottl.Optional[bool], cs ottl.Optional[string]) (ottl.ExprFunc[K], error) {
	conflictStrategy := renameConflictUpsert
	if !cs.IsEmpty() {
		conflictStrategy = cs.Get()
	}

	if conflictStrategy != renameConflictInsert &&
		conflictStrategy != renameConflictUpsert &&
		conflictStrategy != renameConflictFail {
		return nil, fmt.Errorf("%v %w, must be %q, %q or %q", conflictStrategy, ErrRenameInvalidConflictStrategy, renameConflictInsert, renameConflictFail, renameConflictUpsert)
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		sourceMap, err := sm.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		// Do not ignore missing field by default
		ignoreMissing := true
		if !im.IsEmpty() {
			ignoreMissing = im.Get()
		}

		// Get value from source_map
		sourceVal, sourceExists := sourceMap.Get(sourceKey)

		// Apply ignore_missing to the source
		if !sourceExists {
			if ignoreMissing {
				// If ignore_missing is true, return
				return nil, nil
			}
			return nil, fmt.Errorf("%v %w, while ignore_missing is false", sourceKey, ErrRenameKeyIsMissing)
		}

		// Apply conflict_strategy to the target
		oldVal, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		oldExists := (oldVal != nil)

		switch conflictStrategy {
		case renameConflictInsert:
			// Noop if target field present
			if oldExists {
				return nil, nil
			}
		case renameConflictUpsert:
			// Overwrite if present or create when missing
		case renameConflictFail:
			// Fail if target field present
			if oldExists {
				return nil, ErrRenameKeyAlreadyExists
			}
		}

		// Save raw value, since the sourceVal gets modified when the key is removed
		rawVal := sourceVal.AsRaw()

		// Remove field from source
		sourceMap.Remove(sourceKey)

		// Set value to target
		target.Set(ctx, tCtx, rawVal)

		return nil, nil
	}, nil
}
