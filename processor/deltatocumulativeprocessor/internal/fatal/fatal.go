// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// fatal scopes panics to a [context.Context] that is canceled when one occurs.
//
// This allows the use of panic for assertions / fatal faults that cannot be
// recovered from without affecting other parts of the application.
//
// Users are expected to:
//   - defer [fatal.Recover] in every goroutine of the same error scope
//   - check [fatal.Failed] or [context.Cause] before performing any work and
//     abort if a fatal fault has occured
package fatal // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/fatal"

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"time"
)

var now = time.Now

// scope defines a fault scope that is canceled on panic
type scope struct {
	fail func(error)
}

type ctxkey struct{}

// Context returns a regular [context.Context] that can also be used to record
// fatal faults using [fatal.Recover].
//
// All operations of this fault scope are expected to check [context.Cause](ctx)
// or [fatal.Failed](ctx) before performing work.
func Context(parent context.Context) (ctx context.Context, cancel context.CancelCauseFunc) {
	ctx, cancel = context.WithCancelCause(parent)
	state := &scope{fail: cancel}
	ctx = context.WithValue(ctx, ctxkey{}, state)
	return ctx, cancel
}

func from(ctx context.Context) *scope {
	if s, ok := ctx.Value(ctxkey{}).(*scope); ok {
		return s
	}

	return &scope{fail: func(error) {
		panic("fatal.Recover must be used with a ctx from fatal.Context")
	}}
}

// Failed reports whether any fatal fault has occured in this fault scope
// If true, no further work must be performed
func Failed(ctx context.Context) bool {
	_, ok := context.Cause(ctx).(Error)
	return ok
}

// Recover consumes any later panic. In such a it prints a stacktrace and
// cancels ctx
//
// Must be deffered at the start of each goroutine of the same fault scope:
//
//	defer fatal.Recover(ctx)
func Recover(ctx context.Context) {
	r := recover()
	if r == nil {
		return
	}

	state := from(ctx)
	state.fail(Error{Time: now()})

	fmt.Println("fatal:", r, "[recovered]")
	os.Stdout.Write(debug.Stack())
	fmt.Println("ceasing operations")
}

// Error signals a fatal fault happened at given Time.
type Error struct {
	Time time.Time
}

func (e Error) Error() string {
	return fmt.Sprintf("ceased operations due to a fatal error at %s", e.Time)
}
