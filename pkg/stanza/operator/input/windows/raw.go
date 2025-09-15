// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// EventRaw is the rendered xml of an event, however, its message is the original XML of the entire event.
type EventRaw struct {
	TimeCreated   TimeCreated `xml:"System>TimeCreated"`
	RenderedLevel string      `xml:"RenderingInfo>Level"`
	Level         string      `xml:"System>Level"`
	Body          string      `xml:"-"`
}

// parseTimestamp will parse the timestamp of the event.
func (e *EventRaw) ParseTimestamp() time.Time {
	if timestamp, err := time.Parse(time.RFC3339Nano, e.TimeCreated.SystemTime); err == nil {
		return timestamp
	}
	return time.Now()
}

// parseRenderedSeverity will parse the severity of the event.
func (e *EventRaw) ParseRenderedSeverity() entry.Severity {
	switch e.RenderedLevel {
	case "":
		return e.ParseSeverity()
	case "Critical":
		return entry.Fatal
	case "Error":
		return entry.Error
	case "Warning":
		return entry.Warn
	case "Information":
		return entry.Info
	default:
		return entry.Default
	}
}

// parseSeverity will parse the severity of the event when RenderingInfo is not populated
func (e *EventRaw) ParseSeverity() entry.Severity {
	switch e.Level {
	case "1":
		return entry.Fatal
	case "2":
		return entry.Error
	case "3":
		return entry.Warn
	case "4":
		return entry.Info
	default:
		return entry.Default
	}
}

func (e *EventRaw) parseBody() string {
	return e.Body
}

// unmarshalEventRaw will unmarshal EventRaw from xml bytes.
func UnmarshalEventRaw(bytes []byte) (EventRaw, error) {
	var eventRaw EventRaw
	if err := xml.Unmarshal(bytes, &eventRaw); err != nil {
		return EventRaw{}, fmt.Errorf("failed to unmarshal xml bytes into event: %w (%s)", err, string(bytes))
	}
	eventRaw.Body = string(bytes)
	return eventRaw, nil
}
