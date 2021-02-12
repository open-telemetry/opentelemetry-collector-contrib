// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package groupbyauthprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestRingBufferCapacity(t *testing.T) {
	// prepare
	buffer := newRingBuffer(5)

	// test
	tokens := []string{

		pdata.NewTraceID([16]byte{1, 2, 3, 4}).HexString(),
		pdata.NewTraceID([16]byte{2, 3, 4, 5}).HexString(),
		pdata.NewTraceID([16]byte{3, 4, 5, 6}).HexString(),
		pdata.NewTraceID([16]byte{4, 5, 6, 7}).HexString(),
		pdata.NewTraceID([16]byte{5, 6, 7, 8}).HexString(),
		pdata.NewTraceID([16]byte{6, 7, 8, 9}).HexString(),
	}
	for _, token := range tokens {
		buffer.put(token)
	}

	// verify
	for i := 5; i > 0; i-- { // last 5 traces
		token := tokens[i]
		assert.True(t, buffer.contains(token))
	}

	// the first trace should have been evicted
	assert.False(t, buffer.contains(tokens[0]))
}

func TestDeleteFromBuffer(t *testing.T) {
	// prepare
	buffer := newRingBuffer(2)
	token := pdata.NewTraceID([16]byte{1, 2, 3, 4}).HexString()
	buffer.put(token)

	// test
	deleted := buffer.delete(token)

	// verify
	assert.True(t, deleted)
	assert.False(t, buffer.contains(token))
}

func TestDeleteNonExistingFromBuffer(t *testing.T) {
	// prepare
	buffer := newRingBuffer(2)
	token := pdata.NewTraceID([16]byte{1, 2, 3, 4}).HexString()

	// test
	deleted := buffer.delete(token)

	// verify
	assert.False(t, deleted)
	assert.False(t, buffer.contains(token))
}
