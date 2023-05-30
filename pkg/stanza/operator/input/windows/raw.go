// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

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
	bytes         []byte
}

// parseTimestamp will parse the timestamp of the event.
func (e *EventRaw) parseTimestamp() time.Time {
	if timestamp, err := time.Parse(time.RFC3339Nano, e.TimeCreated.SystemTime); err == nil {
		return timestamp
	}
	return time.Now()
}

// parseRenderedSeverity will parse the severity of the event.
func (e *EventRaw) parseRenderedSeverity() entry.Severity {
	switch e.RenderedLevel {
	case "":
		return e.parseSeverity()
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
func (e *EventRaw) parseSeverity() entry.Severity {
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

// parseBody will parse a body from the event.
func (e *EventRaw) parseBody() []byte {
	return e.bytes
}

// unmarshalEventRaw will unmarshal EventRaw from xml bytes.
func unmarshalEventRaw(bytes []byte) (EventRaw, error) {
	var eventRaw EventRaw
	if err := xml.Unmarshal(bytes, &eventRaw); err != nil {
		return EventRaw{}, fmt.Errorf("failed to unmarshal xml bytes into event: %w (%s)", err, string(bytes))
	}
	eventRaw.bytes = bytes
	return eventRaw, nil
}
