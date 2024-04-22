// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expo // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"

type ord interface {
	int | int32 | float64
}

// HiLo returns the greater of a and b by comparing the result of applying fn to
// each
func HiLo[T any, N ord](a, b T, fn func(T) N) (hi, lo T) {
	an, bn := fn(a), fn(b)
	if an > bn {
		return a, b
	}
	return b, a
}
