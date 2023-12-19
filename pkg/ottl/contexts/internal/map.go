// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func GetMapValue[K any](ctx context.Context, tCtx K, m pcommon.Map, key ottl.Key[K]) (any, error) {
	if key == nil {
		return nil, fmt.Errorf("cannot get map value without key")
	}

	s, err := key.String(ctx, tCtx)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("non-string indexing is not supported")
	}

	val, ok := m.Get(*s)
	if !ok {
		return nil, nil
	}

	return getIndexableValue[K](ctx, tCtx, val, key.Next())
}

func SetMapValue[K any](ctx context.Context, tCtx K, m pcommon.Map, key ottl.Key[K], val any) error {
	if key == nil {
		return fmt.Errorf("cannot set map value without key")
	}

	s, err := key.String(ctx, tCtx)
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("non-string indexing is not supported")
	}

	currentValue, ok := m.Get(*s)
	if !ok {
		currentValue = m.PutEmpty(*s)
	}
	return setIndexableValue[K](ctx, tCtx, currentValue, val, key.Next())
}
