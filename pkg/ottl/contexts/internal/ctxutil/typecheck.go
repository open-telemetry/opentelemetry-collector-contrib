// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"

import "fmt"

// ExpectType ensures val can be asserted to T, returning a descriptive error when it cannot.
func ExpectType[T any](val any) (T, error) {
	var zero T
	typed, ok := val.(T)
	if !ok {
		return zero, fmt.Errorf("expects %T but got %T", zero, val)
	}
	return typed, nil
}
