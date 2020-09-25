// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package requestcounter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextWithRequestCounter(t *testing.T) {
	parent := ContextWithRequestCounter(context.Background())
	assert.True(t, counterExists(parent), "parent context contains counter")
	assert.Equal(t, uint32(0), GetRequestCount(parent), "parent context with counter is initialized to 0")

	// ensure counter can be incremented
	IncrementRequestCount(parent)
	assert.Equal(t, uint32(1), GetRequestCount(parent), "parent context is incremented")

	// ensure child contexts retains the counter
	child, cancel := context.WithCancel(parent)
	defer cancel()
	assert.True(t, counterExists(parent), "child context contains counter")
	assert.Equal(t, uint32(1), GetRequestCount(child), "child context carried over parent count")

	// ensure increment on child also increments parent
	IncrementRequestCount(child)
	assert.Equal(t, uint32(2), GetRequestCount(child), "child context can be incremented")
	assert.Equal(t, uint32(2), GetRequestCount(parent), "parent context was incremented when child was incremented")

	// ensure increment on parent also increments child
	IncrementRequestCount(parent)
	assert.Equal(t, uint32(3), GetRequestCount(parent), "parent context can still still increment counter")
	assert.Equal(t, uint32(3), GetRequestCount(child), "child context counter was incremented when parent was incremented")

	assert.Equal(t, uint32(3), GetRequestCount(ContextWithRequestCounter(parent)), "trying to get a context with a counter shouldn't not overwrite an existing counter")

	// ensure counter can be reset
	ResetRequestCount(child)
	assert.Equal(t, uint32(0), GetRequestCount(parent), "parent context counter was reset")
	assert.Equal(t, uint32(0), GetRequestCount(child), "child context counter was reset")

	// ensure no error when context with out counter is passed in to functions
	todo := context.TODO()
	assert.False(t, counterExists(todo), "plain context shouldn't have a counter")
	assert.Equal(t, uint32(0), GetRequestCount(todo), "plain context should return count of 0")
	assert.NotPanics(t, func() { IncrementRequestCount(todo) }, todo, "incrementing a plain counter should not panic")
	assert.Equal(t, uint32(0), GetRequestCount(todo), "incrementing a plain counter should do nothing")
}
