// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package context

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStdLibAssertions(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ch := ctx.Done()

	op := NewOptimised(ctx).(*optimised)

	assert.Equal(t, ch, op.done, "Must have stored the done channel that was returned from ctx.Done")
	assert.Equal(t, ch, op.Done(), "Must have stored the done channel that was returned from ctx.Done")

}

func TestContextTree(t *testing.T) {
	t.Parallel()

	parent, pDone := WithCancel(Background())

	op := NewOptimised(parent)

	child, cDone := WithCancel(op)
	t.Cleanup(cDone)

	pDone()
	assert.ErrorIs(t, child.Err(), context.Canceled)

}

// ch exists to ensure that the compiler
// doesn't optimise the operation out within the benchmark
var ch <-chan struct{}

func BenchmarkBackground(b *testing.B) {
	ctx := Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ch = ctx.Done()
	}
}

func BenchmarkStandardCanceller(b *testing.B) {
	ctx, cancel := WithCancel(context.Background())
	b.Cleanup(cancel)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ch = ctx.Done()
	}
}

func BenchmarkOptimisedCanceller(b *testing.B) {
	ctx, cancel := WithCancel(context.Background())
	b.Cleanup(cancel)
	ctx = NewOptimised(ctx)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ch = ctx.Done()
	}
}
