// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remoteobserverprocessor

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
