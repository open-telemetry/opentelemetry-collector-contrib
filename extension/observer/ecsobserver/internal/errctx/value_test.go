// Copyright The OpenTelemetry Authors
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

package errctx

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithValue(t *testing.T) {
	assert.Nil(t, WithValue(nil, "a", "b"))
	assert.Panics(t, func() {
		_ = WithValue(fmt.Errorf("base"), "", nil)
	})

	e1 := WithValue(fmt.Errorf("base"), "a", "b")
	assert.Equal(t, "base a=b", e1.Error())
	v1, ok := ValueFrom(e1, "a")
	assert.True(t, ok)
	assert.Equal(t, "b", v1)

	// No exists
	v2, ok := ValueFrom(e1, "404")
	assert.False(t, ok)
	assert.Nil(t, v2)

	// Allow nil
	e2 := WithValue(e1, "nilval", nil)
	v3, ok := ValueFrom(e2, "nilval")
	assert.True(t, ok)
	assert.Nil(t, v3)
}

func TestWithValues(t *testing.T) {
	assert.Nil(t, WithValues(nil, map[string]interface{}{"a": "b"}))
	assert.Panics(t, func() {
		_ = WithValues(fmt.Errorf("base"), map[string]interface{}{"": "123"})
	})

	e1 := WithValues(fmt.Errorf("base"), map[string]interface{}{"a": "b", "c": 123})
	// NOTE: we sort the key in the impl so the test is not flaky
	assert.Equal(t, "base a=b c=123", e1.Error())
	v1, ok := ValueFrom(e1, "a")
	assert.True(t, ok)
	assert.Equal(t, "b", v1)

	v2, ok := ValueFrom(e1, "404")
	assert.False(t, ok)
	assert.Nil(t, v2)
}

func TestValueFrom(t *testing.T) {
	v, ok := ValueFrom(nil, "a")
	assert.Nil(t, v)
	assert.False(t, ok)

	// Chained with value in the middle
	t.Run("chained", func(t *testing.T) {
		e1 := fmt.Errorf("base")
		e2 := WithValue(e1, "a", "b")
		e3 := fmt.Errorf("l2 %w", e2)

		v, ok = ValueFrom(e3, "a")
		assert.True(t, ok)
		assert.Equal(t, "b", v)
	})

	// When there is duplication in the chain, the first one comes out got picked up.
	t.Run("duplication", func(t *testing.T) {
		e1 := fmt.Errorf("base")
		e2 := WithValue(e1, "a", "e2")
		e3 := fmt.Errorf("e3 %w", e2)
		e4 := WithValue(e3, "a", "e4")

		v, ok = ValueFrom(e4, "a")
		assert.True(t, ok)
		assert.Equal(t, "e4", v)

		v, ok = ValueFrom(e2, "a")
		assert.True(t, ok)
		assert.Equal(t, "e2", v)
	})

}
