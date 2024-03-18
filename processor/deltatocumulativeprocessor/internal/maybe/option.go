// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maybe

// Ptr is a pointer that points to something that is not guaranteed to exist.
// Any use must "Try()" accessing the underlying value, enforcing checking the
// ok return value too.
// This provides a clear distinction between "not set" and "set to nil"
type Ptr[T any] struct {
	to *T
	ok bool
}

func None[T any]() Ptr[T] {
	return Ptr[T]{to: nil, ok: false}
}

func Some[T any](ptr *T) Ptr[T] {
	return Ptr[T]{to: ptr, ok: true}
}

func (ptr Ptr[T]) Try() (_ *T, ok bool) {
	return ptr.to, ptr.ok
}
