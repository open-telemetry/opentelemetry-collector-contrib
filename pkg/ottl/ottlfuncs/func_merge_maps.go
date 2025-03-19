// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	INSERT = "insert"
	UPDATE = "update"
	UPSERT = "upsert"
)

type MergeMapsArguments[K any] struct {
	Target   ottl.PMapGetter[K]
	Source   ottl.Optional[ottl.PMapGetter[K]]
	Strategy ottl.Optional[string]
	Sources  ottl.Optional[ottl.PMapSliceGetter[K]]
}

func NewMergeMapsFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("merge_maps", &MergeMapsArguments[K]{}, createMergeMapsFunction[K])
}

func createMergeMapsFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*MergeMapsArguments[K])

	if !ok {
		return nil, errors.New("MergeMapsFactory args must be of type *MergeMapsArguments[K]")
	}

	return mergeMaps(args.Target, args.Source, args.Strategy, args.Sources)
}

// mergeMaps function merges the source map and/or the sources map slice into the target map using the supplied strategy to handle conflicts.
// source and sources parameters can be defined separately, but also together.
// Strategy definitions:
//
//	insert: Insert the value from `source` into `target` where the key does not already exist.
//	update: Update the entry in `target` with the value from `source` where the key does exist
//	upsert: Performs insert or update. Insert the value from `source` into `target` where the key does not already exist and update the entry in `target` with the value from `source` where the key does exist.
func mergeMaps[K any](target ottl.PMapGetter[K], source ottl.Optional[ottl.PMapGetter[K]], strategy ottl.Optional[string], sources ottl.Optional[ottl.PMapSliceGetter[K]]) (ottl.ExprFunc[K], error) {
	mergeStrategy := "upsert"
	if !strategy.IsEmpty() {
		mergeStrategy = strategy.Get()
	}
	if mergeStrategy != INSERT && mergeStrategy != UPDATE && mergeStrategy != UPSERT {
		return nil, fmt.Errorf("invalid value for strategy, %v, must be 'insert', 'update' or 'upsert'", strategy)
	}

	if source.IsEmpty() && sources.IsEmpty() {
		return nil, fmt.Errorf("at least one of the optional arguments ('source' or 'sources') must be provided")
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		targetMap, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if !source.IsEmpty() {
			valueMap, err := source.Get().Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}

			if err := merge(mergeStrategy, &valueMap, &targetMap); err != nil {
				return nil, err
			}
		}

		if !sources.IsEmpty() {
			valueMapSlice, err := sources.Get().Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}

			for _, val := range valueMapSlice {
				if err := merge(mergeStrategy, &val, &targetMap); err != nil {
					return nil, err
				}
			}
		}

		return nil, nil
	}, nil
}

func merge(strategy string, source *pcommon.Map, target *pcommon.Map) error {
	switch strategy {
	case INSERT:
		for k, v := range source.All() {
			if _, ok := target.Get(k); !ok {
				tv := target.PutEmpty(k)
				v.CopyTo(tv)
			}
		}
	case UPDATE:
		for k, v := range source.All() {
			if tv, ok := target.Get(k); ok {
				v.CopyTo(tv)
			}
		}
	case UPSERT:
		for k, v := range source.All() {
			tv := target.PutEmpty(k)
			v.CopyTo(tv)
		}
	default:
		return fmt.Errorf("unknown strategy, %v", strategy)
	}

	return nil
}
