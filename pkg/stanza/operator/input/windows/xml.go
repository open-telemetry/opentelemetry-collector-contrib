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

// +build windows

package windows // import "github.com/open-telemetry/opentelemetry-log-collection/operator/input/windows"

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
)

// EventXML is the rendered xml of an event.
type EventXML struct {
	EventID     EventID     `xml:"System>EventID"`
	Provider    Provider    `xml:"System>Provider"`
	Computer    string      `xml:"System>Computer"`
	Channel     string      `xml:"System>Channel"`
	RecordID    uint64      `xml:"System>EventRecordID"`
	TimeCreated TimeCreated `xml:"System>TimeCreated"`
	Message     string      `xml:"RenderingInfo>Message"`
	Level       string      `xml:"RenderingInfo>Level"`
	Task        string      `xml:"RenderingInfo>Task"`
	Opcode      string      `xml:"RenderingInfo>Opcode"`
	Keywords    []string    `xml:"RenderingInfo>Keywords>Keyword"`
}

// parseTimestamp will parse the timestamp of the event.
func (e *EventXML) parseTimestamp() time.Time {
	if timestamp, err := time.Parse(time.RFC3339Nano, e.TimeCreated.SystemTime); err == nil {
		return timestamp
	}
	return time.Now()
}

// parseSeverity will parse the severity of the event.
func (e *EventXML) parseSeverity() entry.Severity {
	switch e.Level {
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

// parseBody will parse a body from the event.
func (e *EventXML) parseBody() map[string]interface{} {
	message, details := e.parseMessage()
	body := map[string]interface{}{
		"event_id": map[string]interface{}{
			"qualifiers": e.EventID.Qualifiers,
			"id":         e.EventID.ID,
		},
		"provider": map[string]interface{}{
			"name":         e.Provider.Name,
			"guid":         e.Provider.GUID,
			"event_source": e.Provider.EventSourceName,
		},
		"system_time": e.TimeCreated.SystemTime,
		"computer":    e.Computer,
		"channel":     e.Channel,
		"record_id":   e.RecordID,
		"level":       e.Level,
		"message":     message,
		"task":        e.Task,
		"opcode":      e.Opcode,
		"keywords":    e.Keywords,
	}
	if len(details) > 0 {
		body["details"] = details
	}
	return body
}

// parseMessage will attempt to parse a message into a message and details
func (e *EventXML) parseMessage() (string, map[string]interface{}) {
	switch e.Channel {
	case "Security":
		return parseSecurity(e.Message)
	default:
		return e.Message, nil
	}
}

// unmarshalEventXML will unmarshal EventXML from xml bytes.
func unmarshalEventXML(bytes []byte) (EventXML, error) {
	var eventXML EventXML
	if err := xml.Unmarshal(bytes, &eventXML); err != nil {
		return EventXML{}, fmt.Errorf("failed to unmarshal xml bytes into event: %w (%s)", err, string(bytes))
	}
	return eventXML, nil
}

// EventID is the identifier of the event.
type EventID struct {
	Qualifiers uint16 `xml:"Qualifiers,attr"`
	ID         uint32 `xml:",chardata"`
}

// TimeCreated is the creation time of the event.
type TimeCreated struct {
	SystemTime string `xml:"SystemTime,attr"`
}

// Provider is the provider of the event.
type Provider struct {
	Name            string `xml:"Name,attr"`
	GUID            string `xml:"Guid,attr"`
	EventSourceName string `xml:"EventSourceName,attr"`
}
