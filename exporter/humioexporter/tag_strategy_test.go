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

package humioexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newStructuredEvent(attrs interface{}) *HumioStructuredEvent {
	return &HumioStructuredEvent{
		Timestamp:  time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC),
		AsUnix:     false,
		Attributes: attrs,
	}
}

func TestConsume(t *testing.T) {
	// Arrange
	organizer := newTagOrganizer(TagServiceName, func(evt *HumioStructuredEvent, t Tagger) string {
		return evt.Attributes.(string)
	})

	// Act
	organizer.consume(newStructuredEvent("a"))
	organizer.consume(newStructuredEvent("b"))
	organizer.consume(newStructuredEvent("c"))
	organizer.consume(newStructuredEvent("a"))
	organizer.consume(newStructuredEvent("b"))

	// Assert
	assert.Len(t, organizer.evtsByTag, 3)
	assert.Len(t, organizer.evtsByTag["a"], 2)
	assert.Len(t, organizer.evtsByTag["b"], 2)
	assert.Len(t, organizer.evtsByTag["c"], 1)
}

func TestAsEvents(t *testing.T) {
	// Arrange
	organizer := newTagOrganizer(TagTraceID, func(evt *HumioStructuredEvent, t Tagger) string {
		return evt.Attributes.(string)
	})

	a := newStructuredEvent("123")
	b := newStructuredEvent("456")
	c := newStructuredEvent("")
	d := newStructuredEvent("456")

	organizer.consume(a)
	organizer.consume(b)
	organizer.consume(c)
	organizer.consume(d)

	expected := []*HumioStructuredEvents{
		{
			Tags:   map[string]string{"trace_id": "123"},
			Events: []*HumioStructuredEvent{a},
		},
		{
			Tags:   map[string]string{"trace_id": "456"},
			Events: []*HumioStructuredEvent{b, d},
		},
		{
			Events: []*HumioStructuredEvent{c},
		},
	}

	// Act
	actual := organizer.asEvents()

	// Assert
	assert.Len(t, actual, 3)
	assert.Contains(t, actual, expected[0])
	assert.Contains(t, actual, expected[1])
	assert.Contains(t, actual, expected[2])
}
