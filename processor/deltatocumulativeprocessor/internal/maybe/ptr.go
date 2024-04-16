// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// maybe provides utilities for representing data may or may not exist at
// runtime in a safe way.
//
// A typical approach to this are pointers, but they suffer from two issues:
//   - Unsafety: permitting nil pointers must require careful checking on each use,
//     which is easily forgotten
//   - Blindness: nil itself does cannot differentiate between "set to nil" and
//     "not set all", leading to unexepcted edge cases
//
// The [Ptr] type of this package provides a safe alternative with a clear
// distinction between "not set" and "set to nil".
package maybe // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maybe"

// Ptr references some value of type T that is not guaranteed to exist.
// Callers must use [Ptr.Try] to access the underlying value, checking the
// ok return value too.
// This provides a clear distinction between "not set" and "set to nil".
//
// Use [Some] and [None] to create Ptrs.
type Ptr[T any] struct {
	to *T
	ok bool
}

// None returns a Ptr that represents "not-set".
// This is equal to a zero-value Ptr.
func None[T any]() Ptr[T] {
	return Ptr[T]{to: nil, ok: false}
}

// Some returns a pointer to the passed T.
//
// The ptr argument may be nil, in which case this represents "explicitly set to
// nil".
func Some[T any](ptr *T) Ptr[T] {
	return Ptr[T]{to: ptr, ok: true}
}

// Try attempts to de-reference the Ptr, giving one of three results:
//
//   - nil, false: not-set
//   - nil, true: explicitly set to nil
//   - non-nil, true: set to some value
//
// This provides extra safety over bare pointers, because callers are forced by
// the compiler to either check or explicitly ignore the ok value.
func (ptr Ptr[T]) Try() (_ *T, ok bool) {
	return ptr.to, ptr.ok
}
