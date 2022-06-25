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

import "context"

type (
	Context    = context.Context
	CancelFunc = context.CancelFunc

	// optimised is a lock free implementation of context.Context.
	// Within the context.Done() code, there is a lock for creating
	// and returning the channel, however, it is permisable to store
	// the resulting done channel to be used later on since the
	// same channel is always returned.
	// This types stores the resulting done channel with the assumption that
	// it would be used heavily.
	// All other methods are handled by the original context that was passed in.
	//
	// Please see benchmark results for justification
	optimised struct {
		context.Context

		done <-chan struct{}
	}
)

var _ Context = (*optimised)(nil)

// NewOptimised returns a context that has an already computed
// done channel that allows for speed of select processing
// or checking if the context is done.
func NewOptimised(ctx context.Context) context.Context {
	return &optimised{ctx, ctx.Done()}
}

func (o *optimised) Done() <-chan struct{} {
	return o.done
}

// All of the std lib functions that would normally be exported
// within the standard lib
var (
	Background = context.Background
	TODO       = context.TODO

	WithCancel   = context.WithCancel
	WithTimeout  = context.WithTimeout
	WithDeadline = context.WithDeadline
	WithValue    = context.WithValue
)
