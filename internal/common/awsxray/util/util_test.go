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

package util

import (
	"testing"

	"gotest.tools/assert"
)

func TestString(t *testing.T) {
	v := "something"
	vPtr := String(v)
	assert.Equal(t, v, *vPtr, "should be equal")
}

func TestBool(t *testing.T) {
	v := true
	vPtr := Bool(v)
	assert.Equal(t, v, *vPtr, "should be equal")
}

func TestInt64(t *testing.T) {
	v := int64(30)
	vPtr := Int64(v)
	assert.Equal(t, v, *vPtr, "should be equal")
}

func TestInt(t *testing.T) {
	v := 3
	vPtr := Int(v)
	assert.Equal(t, v, *vPtr, "should be equal")
}

func TestFloat64(t *testing.T) {
	v := float64(4.5)
	vPtr := Float64(v)
	assert.Equal(t, v, *vPtr, "should be equal")
}
