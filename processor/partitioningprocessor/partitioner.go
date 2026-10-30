// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/partitioningprocessor"

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// keyValue holds the result of evaluating a single partition key expression.
// isNil is true when the expression returned no value (e.g. a missing
// attribute); in that case value is the zero string and the key is omitted
// from the outgoing request metadata.
type keyValue struct {
	value string
	isNil bool
}

// partitioned holds a partition key and the signal data belonging to it.
type partitioned[T any] struct {
	values []keyValue
	data   T
}

type closableOTTLContext interface {
	Close()
}

// evaluateStringExpressions evaluates each expression against tCtx and writes
// the resulting values, in order, into dst (which must have len(exprs)).
// A nil OTTL result (e.g. a missing attribute) is stored as
// keyValue{isNil: true} so callers can distinguish it from an explicit empty
// string. Any non-nil, non-string result is an error.
func evaluateStringExpressions[K closableOTTLContext](ctx context.Context, exprs []*ottl.ValueExpression[K], tCtx K, dst []keyValue) error {
	defer tCtx.Close()

	for i, expr := range exprs {
		val, err := expr.Eval(ctx, tCtx)
		if err != nil {
			return fmt.Errorf("evaluating key at index %d: %w", i, err)
		}
		switch v := val.(type) {
		case nil:
			dst[i] = keyValue{isNil: true}
		case string:
			dst[i] = keyValue{value: v}
		default:
			return fmt.Errorf("key at index %d: expected value expression to evaluate to a string, got %T", i, val)
		}
	}
	return nil
}

// appendPartitionKey appends a collision-free map key derived from values to
// b. Each element is encoded as:
//
//   - '0'               — nil (expression returned no value)
//   - '1' <len> ':' <v> — non-nil string
//
// Because nil encodes to a single '0' and non-nil always starts with '1',
// the encoding is unambiguous regardless of the string content.
func appendPartitionKey(b []byte, values []keyValue) []byte {
	for _, v := range values {
		if v.isNil {
			b = append(b, '0')
			continue
		}
		b = append(b, '1')
		b = strconv.AppendInt(b, int64(len(v.value)), 10)
		b = append(b, ':')
		b = append(b, v.value...)
	}
	return b
}

// grouper assigns items to partitions in first-seen order. It reuses its
// scratch buffers across items so that only a new partition allocates.
type grouper struct {
	scratch []keyValue
	key     []byte
	lastKey []byte
	lastID  int
	index   map[string]int
	values  [][]keyValue // evaluated key values, indexed by partition ID
}

func newGrouper(numKeys int) *grouper {
	return &grouper{
		scratch: make([]keyValue, numKeys),
		lastID:  -1,
		index:   make(map[string]int),
	}
}

// assign evaluates exprs against tCtx and returns the partition ID of the item.
func assign[K closableOTTLContext](ctx context.Context, g *grouper, exprs []*ottl.ValueExpression[K], tCtx K) (int, error) {
	if err := evaluateStringExpressions(ctx, exprs, tCtx, g.scratch); err != nil {
		return 0, err
	}
	g.key = appendPartitionKey(g.key[:0], g.scratch)
	// Consecutive items usually share a partition; skip the map lookup.
	if g.lastID >= 0 && bytes.Equal(g.key, g.lastKey) {
		return g.lastID, nil
	}
	id, ok := g.index[string(g.key)]
	if !ok {
		id = len(g.values)
		g.index[string(g.key)] = id
		g.values = append(g.values, slices.Clone(g.scratch))
	}
	g.lastKey = append(g.lastKey[:0], g.key...)
	g.lastID = id
	return id, nil
}

// single returns data as the only partition when every item was assigned to
// the same partition, avoiding any copy. ok is false when there is more
// than one partition.
func single[T any](g *grouper, data T) (parts []partitioned[T], ok bool) {
	switch len(g.values) {
	case 0:
		return nil, true
	case 1:
		return []partitioned[T]{{values: g.values[0], data: data}}, true
	}
	return nil, false
}

// newPartitions returns one empty partition per group, in first-seen order.
func newPartitions[T any](g *grouper, newData func() T) []partitioned[T] {
	parts := make([]partitioned[T], len(g.values))
	for i, v := range g.values {
		parts[i] = partitioned[T]{values: v, data: newData()}
	}
	return parts
}
