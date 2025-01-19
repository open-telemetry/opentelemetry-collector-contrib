// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestReceiverMap(t *testing.T) {
	rm := receiverMap{}
	assert.Equal(t, 0, rm.Size())

	r1 := &nopWithEndpointReceiver{}
	r2 := &nopWithEndpointReceiver{}
	r3 := &nopWithEndpointReceiver{}

	rm.Put("a", r1)
	assert.Equal(t, 1, rm.Size())

	rm.Put("a", r2)
	assert.Equal(t, 2, rm.Size())

	rm.Put("b", r3)
	assert.Equal(t, 3, rm.Size())

	assert.Equal(t, []component.Component{r1, r2}, rm.Get("a"))
	assert.Nil(t, rm.Get("missing"))

	rm.RemoveAll("missing")
	assert.Equal(t, 3, rm.Size())

	rm.RemoveAll("b")
	assert.Equal(t, 2, rm.Size())

	rm.RemoveAll("a")
	assert.Equal(t, 0, rm.Size())

	rm.Put("a", r1)
	rm.Put("b", r2)
	assert.Equal(t, 2, rm.Size())
	assert.Equal(t, []component.Component{r1, r2}, rm.Values())
}
