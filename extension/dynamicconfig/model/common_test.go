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

package model

import (
	"bytes"
	"testing"
)

func TestCombineHash(t *testing.T) {
	chunks := [][]byte{
		[]byte{0x55, 0x55, 0x55, 0x55},
		[]byte{0xAA, 0xAA, 0xAA, 0xAA},
	}

	expected := []byte{0xFF, 0xFF, 0xFF, 0xFF}
	result := combineHash(chunks)

	if !bytes.Equal(expected, result) {
		t.Errorf("expected: %v, got: %v", expected, result)
	}
}

func TestCombineHashPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("combineHash did not panic")
		}
	}()

	chunks := [][]byte{
		[]byte{0x55, 0x55, 0x55, 0x55},
		[]byte{0xAA, 0xAA},
	}

	combineHash(chunks)
}
