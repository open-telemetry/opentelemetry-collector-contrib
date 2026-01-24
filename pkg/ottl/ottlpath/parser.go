// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlpath // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlpath"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"
)

// Path represents a chain of path parts in an OTTL statement, such as `body.string`.
// A Path has a name, and potentially a set of keys.
// If the path in the OTTL statement contains multiple parts (separated by a dot (`.`)), then the Path will have a pointer to the next Path.
type Path[K any] interface {
	// Context is the OTTL context name of this Path.
	Context() string

	// Name is the name of this segment of the path.
	Name() string

	// Next provides the next path segment for this Path.
	// Will return nil if there is no next path.
	Next() Path[K]

	// Keys provides the Keys for this Path.
	// Will return nil if there are no Keys.
	Keys() []Key[K]

	// String returns a string representation of this Path and the next Paths
	String() string
}

// Key represents a chain of keys in an OTTL statement, such as `attributes["foo"]["bar"]`.
// A Key has a String or Int, and potentially the next Key.
// If the path in the OTTL statement contains multiple keys, then the Key will have a pointer to the next Key.
type Key[K any] interface {
	// String returns a pointer to the Key's string value.
	// If the Key does not have a string value the returned value is nil.
	// If Key experiences an error retrieving the value it is returned.
	String(context.Context, K) (*string, error)

	// Int returns a pointer to the Key's int value.
	// If the Key does not have a int value the returned value is nil.
	// If Key experiences an error retrieving the value it is returned.
	Int(context.Context, K) (*int64, error)

	// ExpressionGetter returns a Getter to the expression, that can be
	// part of the path.
	// If the Key does not have an expression the returned value is nil.
	// If Key experiences an error retrieving the value it is returned.
	ExpressionGetter(context.Context, K) (ottlruntime.Getter[K, any], error)
}

// PathExpressionParser is how a context provides OTTL access to all its Paths.
type PathExpressionParser[K any] func(Path[K]) (ottlruntime.GetSetter[K, any], error)
