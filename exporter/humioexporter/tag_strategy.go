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

// Definition of tagging strategies
type tag string

const (
	// TagNone disables tagging of payloads to Humio
	TagNone tag = "none"

	// TagTraceID tags payloads to Humio using the trace ID, or an empty string if
	// the trace ID is missing
	TagTraceID tag = "trace_id"

	// TagServiceName tags payloads to Humio using the service name, or an empty
	// string if it is missing
	TagServiceName tag = "service_name"
)

// Tagger represents a tagging strategy understood by the Humio exporter
type Tagger interface {
	Tag() tag
}

// Tag returns the underlying representation of the tagging strategy. This
// exists to ensure that strategies implement the Tagger interface
func (t tag) Tag() tag {
	return t
}

// TagOrganizer represents a type for organizing HumioStructuredEvents by tag
// while a tree of structured data from OpenTelemetry is being converted into
// Humio's format. Depending on the tagging strategy, the structure of
// resource- and instrumentation logs/spans is not necessarily optimal for
// sending to Humio
type TagOrganizer struct {
	evtsByTag map[string][]*HumioStructuredEvent
	strategy  Tagger

	// getTag should return the tag associated with a certain event, adhering to
	// the specified taggin strategy
	getTag func(*HumioStructuredEvent, Tagger) string
}

func newTagOrganizer(strategy Tagger, getTag func(*HumioStructuredEvent, Tagger) string) *TagOrganizer {
	return &TagOrganizer{
		evtsByTag: make(map[string][]*HumioStructuredEvent),
		strategy:  strategy,
		getTag:    getTag,
	}
}

// Store an event in the group corresponding to the tag of the event. The tag is
// retrieved using the getTag() callback function, which should adhere to the
// tagging strategy
func (t *TagOrganizer) consume(evt *HumioStructuredEvent) {
	tag := t.getTag(evt, t.strategy)

	if group, ok := t.evtsByTag[tag]; ok {
		t.evtsByTag[tag] = append(group, evt)
	} else {
		t.evtsByTag[tag] = []*HumioStructuredEvent{evt}
	}
}

// Converts the currently stored groups into a slice of structured events ready
// to send to Humio
func (t *TagOrganizer) asEvents() []*HumioStructuredEvents {
	evts := make([]*HumioStructuredEvents, 0, len(t.evtsByTag))

	for tag, group := range t.evtsByTag {
		if tag == "" {
			evts = append(evts, &HumioStructuredEvents{Events: group})
		} else {
			evts = append(evts, &HumioStructuredEvents{
				Tags: map[string]string{
					string(t.strategy.Tag()): tag,
				},
				Events: group,
			})
		}
	}

	return evts
}
