// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package admission // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/admission"

import (
	"context"
)

// Queue is a weighted admission queue interface.
type Queue interface {
	// Acquire asks the controller to admit the caller.
	//
	// The weight parameter specifies how large of an admission to make.
	// This might be used on the bytes of request (for example) to differentiate
	// between large and small requests.
	//
	// Admit will return when one of the following events occurs:
	//
	//   (1) admission is allowed, or
	//   (2) the provided ctx becomes canceled, or
	//   (3) there are so many existing waiters that the
	//       controller decides to reject this caller without
	//       admitting it.
	//
	// In case (1), the return value will be a non-nil error. The
	// caller must invoke Release(weight) after it is finished
	// with the resource being guarded by the admission
	// controller.
	//
	// In case (2), the return value will be a Cancelled or
	// DeadlineExceeded error.
	//
	// In case (3), the return value will be a ResourceExhausted
	// error.
	Acquire(ctx context.Context, weight int64) error

	// Release will be eliminated as part of issue #36074.
	Release(weight int64) error
}

type noopController struct{}

var _ Queue = noopController{}

// NewUnboundedQueue returns a no-op implementation of the Queue interface.
func NewUnboundedQueue() Queue {
	return noopController{}
}

// Acquire implements Queue.
func (noopController) Acquire(_ context.Context, _ int64) error {
	return nil
}

// Acquire implements Queue.
func (noopController) Release(_ int64) error {
	return nil
}
