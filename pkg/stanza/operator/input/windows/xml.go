// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// EventXML is the rendered xml of an event.
type EventXML struct {
	Original         string       `xml:"-"`
	EventID          EventID      `xml:"System>EventID"`
	Provider         Provider     `xml:"System>Provider"`
	Computer         string       `xml:"System>Computer"`
	Channel          string       `xml:"System>Channel"`
	RecordID         uint64       `xml:"System>EventRecordID"`
	TimeCreated      TimeCreated  `xml:"System>TimeCreated"`
	Message          string       `xml:"RenderingInfo>Message"`
	RenderedLevel    string       `xml:"RenderingInfo>Level"`
	Level            string       `xml:"System>Level"`
	RenderedTask     string       `xml:"RenderingInfo>Task"`
	Task             string       `xml:"System>Task"`
	RenderedOpcode   string       `xml:"RenderingInfo>Opcode"`
	Opcode           string       `xml:"System>Opcode"`
	RenderedKeywords []string     `xml:"RenderingInfo>Keywords>Keyword"`
	Keywords         []string     `xml:"System>Keywords"`
	Security         *Security    `xml:"System>Security"`
	Execution        *Execution   `xml:"System>Execution"`
	EventData        EventData    `xml:"EventData"`
	Correlation      *Correlation `xml:"System>Correlation"`
	Version          uint8        `xml:"System>Version"`
}

// parseTimestamp will parse the timestamp of the event.
func parseTimestamp(ts string) time.Time {
	if timestamp, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return timestamp
	}
	return time.Now()
}

// parseRenderedSeverity will parse the severity of the event.
func parseSeverity(renderedLevel, level string) entry.Severity {
	switch renderedLevel {
	case "":
		switch level {
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

// formattedBody will parse a body from the event.
func formattedBody(e *EventXML) map[string]any {
	message, details := parseMessage(e.Channel, e.Message)

	level := e.RenderedLevel
	if level == "" {
		level = e.Level
	}

	task := e.RenderedTask
	if task == "" {
		task = e.Task
	}

	opcode := e.RenderedOpcode
	if opcode == "" {
		opcode = e.Opcode
	}

	keywords := e.RenderedKeywords
	if keywords == nil {
		keywords = e.Keywords
	}

	body := map[string]any{
		"event_id": map[string]any{
			"qualifiers": e.EventID.Qualifiers,
			"id":         e.EventID.ID,
		},
		"provider": map[string]any{
			"name":         e.Provider.Name,
			"guid":         e.Provider.GUID,
			"event_source": e.Provider.EventSourceName,
		},
		"system_time": e.TimeCreated.SystemTime,
		"computer":    e.Computer,
		"channel":     e.Channel,
		"record_id":   e.RecordID,
		"level":       level,
		"message":     message,
		"task":        task,
		"opcode":      opcode,
		"keywords":    keywords,
		"event_data":  parseEventData(e.EventData),
		"version":     e.Version,
	}

	if len(details) > 0 {
		body["details"] = details
	}

	if e.Security != nil && e.Security.UserID != "" {
		body["security"] = map[string]any{
			"user_id": e.Security.UserID,
		}
	}

	if e.Execution != nil {
		body["execution"] = e.Execution.asMap()
	}

	if e.Correlation != nil {
		body["correlation"] = e.Correlation.asMap()
	}

	return body
}

// parseMessage will attempt to parse a message into a message and details
func parseMessage(channel, message string) (string, map[string]any) {
	switch channel {
	case "Security":
		return parseSecurity(message)
	default:
		return message, nil
	}
}

// parse event data into a map[string]interface
// see: https://learn.microsoft.com/en-us/windows/win32/wes/eventschema-datafieldtype-complextype
func parseEventData(eventData EventData) map[string]any {
	outputMap := make(map[string]any, 3)
	if eventData.Name != "" {
		outputMap["name"] = eventData.Name
	}
	if eventData.Binary != "" {
		outputMap["binary"] = eventData.Binary
	}

	if len(eventData.Data) == 0 {
		return outputMap
	}

	dataMaps := make([]any, len(eventData.Data))
	for i, data := range eventData.Data {
		dataMaps[i] = map[string]any{
			data.Name: data.Value,
		}
	}

	outputMap["data"] = dataMaps

	return outputMap
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

type EventData struct {
	// https://learn.microsoft.com/en-us/windows/win32/wes/eventschema-eventdatatype-complextype
	// ComplexData is not supported.
	Name   string `xml:"Name,attr"`
	Data   []Data `xml:"Data"`
	Binary string `xml:"Binary"`
}

type Data struct {
	// https://learn.microsoft.com/en-us/windows/win32/wes/eventschema-datafieldtype-complextype
	Name  string `xml:"Name,attr"`
	Value string `xml:",chardata"`
}

// Security contains info pertaining to the user triggering the event.
type Security struct {
	UserID string `xml:"UserID,attr"`
}

// Execution contains info pertaining to the process that triggered the event.
type Execution struct {
	// ProcessID and ThreadID are required on execution info
	ProcessID uint `xml:"ProcessID,attr"`
	ThreadID  uint `xml:"ThreadID,attr"`
	// These remaining fields are all optional for execution info
	ProcessorID   *uint `xml:"ProcessorID,attr"`
	SessionID     *uint `xml:"SessionID,attr"`
	KernelTime    *uint `xml:"KernelTime,attr"`
	UserTime      *uint `xml:"UserTime,attr"`
	ProcessorTime *uint `xml:"ProcessorTime,attr"`
}

func (e Execution) asMap() map[string]any {
	result := map[string]any{
		"process_id": e.ProcessID,
		"thread_id":  e.ThreadID,
	}

	if e.ProcessorID != nil {
		result["processor_id"] = *e.ProcessorID
	}

	if e.SessionID != nil {
		result["session_id"] = *e.SessionID
	}

	if e.KernelTime != nil {
		result["kernel_time"] = *e.KernelTime
	}

	if e.UserTime != nil {
		result["user_time"] = *e.UserTime
	}

	if e.ProcessorTime != nil {
		result["processor_time"] = *e.ProcessorTime
	}

	return result
}

// Correlation contains the activity identifiers that consumers can use to group related events together.
type Correlation struct {
	// ActivityID and RelatedActivityID are optional fields
	// https://learn.microsoft.com/en-us/windows/win32/wes/eventschema-correlation-systempropertiestype-element
	ActivityID        *string `xml:"ActivityID,attr"`
	RelatedActivityID *string `xml:"RelatedActivityID,attr"`
}

func (e Correlation) asMap() map[string]any {
	result := map[string]any{}

	if e.ActivityID != nil {
		result["activity_id"] = *e.ActivityID
	}

	if e.RelatedActivityID != nil {
		result["related_activity_id"] = *e.RelatedActivityID
	}

	return result
}

// unmarshalEventXML will unmarshal EventXML from xml bytes.
func unmarshalEventXML(bytes []byte) (*EventXML, error) {
	var eventXML EventXML
	if err := xml.Unmarshal(bytes, &eventXML); err != nil {
		return nil, fmt.Errorf("failed to unmarshal xml bytes into event: %w (%s)", err, string(bytes))
	}
	eventXML.Original = string(bytes)
	return &eventXML, nil
}
