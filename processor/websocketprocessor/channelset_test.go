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

package websocketprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChannelset(t *testing.T) {
	cs := newChannelSet()
	ch := make(chan []byte)
	key := cs.add(ch)
	go func() {
		cs.writeBytes([]byte("hello"))
	}()
	assert.Eventually(t, func() bool {
		return assert.Equal(t, []byte("hello"), <-ch)
	}, time.Second, time.Millisecond*10)
	cs.closeAndRemove(key)
}
