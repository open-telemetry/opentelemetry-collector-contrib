// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// EventXML is the rendered xml of an event.
// See: https://learn.microsoft.com/en-us/windows/win32/wes/eventschema-schema
type EventXML struct {
	Original            string               `xml:"-"`
	EventID             EventID              `xml:"System>EventID"`
	Provider            Provider             `xml:"System>Provider"`
	Computer            string               `xml:"System>Computer"`
	Channel             string               `xml:"System>Channel"`
	RecordID            uint64               `xml:"System>EventRecordID"`
	TimeCreated         TimeCreated          `xml:"System>TimeCreated"`
	Level               string               `xml:"System>Level"`
	Task                string               `xml:"System>Task"`
	Opcode              string               `xml:"System>Opcode"`
	Keywords            []string             `xml:"System>Keywords"`
	Security            *Security            `xml:"System>Security"`
	Execution           *Execution           `xml:"System>Execution"`
	EventData           EventData            `xml:"EventData"`
	UserData            *UserData            `xml:"UserData"`
	Correlation         *Correlation         `xml:"System>Correlation"`
	Version             uint8                `xml:"System>Version"`
	RenderingInfo       *RenderingInfo       `xml:"RenderingInfo"`
	ProcessingErrorData *ProcessingErrorData `xml:"ProcessingErrorData"`
	DebugData           *DebugData           `xml:"DebugData"`
	// BinaryEventData contains raw hex-encoded binary data logged by legacy providers.
	// https://learn.microsoft.com/en-us/windows/win32/wes/eventschema-binaryeventdata-eventtype-element
	BinaryEventData string `xml:"BinaryEventData"`
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
func formattedBody(e *EventXML, eventDataFormat EventDataFormat) map[string]any {
	var rawMessage string
	level := e.Level
	task := e.Task
	opcode := e.Opcode
	keywords := e.Keywords

	if e.RenderingInfo != nil {
		rawMessage = e.RenderingInfo.Message
		if e.RenderingInfo.Level != "" {
			level = e.RenderingInfo.Level
		}
		if e.RenderingInfo.Task != "" {
			task = e.RenderingInfo.Task
		}
		if e.RenderingInfo.Opcode != "" {
			opcode = e.RenderingInfo.Opcode
		}
		if e.RenderingInfo.Keywords != nil {
			keywords = e.RenderingInfo.Keywords
		}
	}

	message, details := parseMessage(e.Channel, rawMessage)

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
		"event_data":  parseEventData(e.EventData, eventDataFormat),
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

	if e.RenderingInfo != nil {
		body["rendering_info"] = e.RenderingInfo.asMap()
	}

	if e.UserData != nil {
		body["user_data"] = e.UserData.asMap()
	}

	if e.ProcessingErrorData != nil {
		body["processing_error_data"] = e.ProcessingErrorData.asMap()
	}

	if e.DebugData != nil {
		body["debug_data"] = e.DebugData.asMap()
	}

	if e.BinaryEventData != "" {
		body["binary_event_data"] = e.BinaryEventData
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

// parseEventData converts EventData XML elements into a map.
// When format is EventDataFormatMap, named Data elements become direct keys and
// anonymous elements use numbered keys (param1, param2, …).
// When format is EventDataFormatArray, data is stored as a "data" slice of
// single-key maps, preserving the original collector format.
// see: https://learn.microsoft.com/en-us/windows/win32/wes/eventschema-datafieldtype-complextype
func parseEventData(eventData EventData, format EventDataFormat) map[string]any {
	outputMap := make(map[string]any, len(eventData.Data)+2)

	if eventData.Name != "" {
		outputMap["name"] = eventData.Name
	}
	if eventData.Binary != "" {
		outputMap["binary"] = eventData.Binary
	}

	if len(eventData.Data) == 0 {
		return outputMap
	}

	switch format {
	case EventDataFormatArray:
		dataMaps := make([]any, len(eventData.Data))
		for i, data := range eventData.Data {
			dataMaps[i] = map[string]any{
				data.Name: data.Value,
			}
		}
		outputMap["data"] = dataMaps
	default:
		anonymousCounter := 1
		for _, data := range eventData.Data {
			if data.Name != "" {
				outputMap[data.Name] = data.Value
			} else {
				key := fmt.Sprintf("param%d", anonymousCounter)
				outputMap[key] = data.Value
				anonymousCounter++
			}
		}
	}

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

// RenderingInfo contains human-readable strings for event fields, populated
// when the event is rendered with a publisher metadata (RenderDeep path).
// https://learn.microsoft.com/en-us/windows/win32/wes/eventschema-renderinginfotype-complextype
type RenderingInfo struct {
	Culture  string   `xml:"Culture,attr"`
	Message  string   `xml:"Message"`
	Level    string   `xml:"Level"`
	Task     string   `xml:"Task"`
	Opcode   string   `xml:"Opcode"`
	Channel  string   `xml:"Channel"`
	Provider string   `xml:"Provider"`
	Keywords []string `xml:"Keywords>Keyword"`
}

func (r RenderingInfo) asMap() map[string]any {
	return map[string]any{
		"culture":  r.Culture,
		"message":  r.Message,
		"level":    r.Level,
		"task":     r.Task,
		"opcode":   r.Opcode,
		"channel":  r.Channel,
		"provider": r.Provider,
		"keywords": r.Keywords,
	}
}

// UserData contains provider-defined event data as an alternative to EventData.
// The structure is arbitrary and defined by each provider's XML manifest.
// https://learn.microsoft.com/en-us/windows/win32/wes/eventschema-userdatatype-complextype
type UserData struct {
	// Name is the local name of the first child element, which identifies the event type.
	Name string
	// Data holds the key-value pairs parsed from the first child element's children.
	Data map[string]string
}

// UnmarshalXML implements xml.Unmarshaler for UserData.
// It reads the first child element and collects its direct children as key-value pairs.
func (u *UserData) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	// Find the first child element of <UserData>, which names the event type.
	var innerStart xml.StartElement
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			innerStart = t
			goto parseChildren
		case xml.EndElement:
			return nil // empty <UserData>
		}
	}

parseChildren:
	u.Name = innerStart.Name.Local
	u.Data = make(map[string]string)

	// Collect direct children of the inner element as key-value pairs.
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var value string
			if err := d.DecodeElement(&value, &t); err != nil {
				return err
			}
			u.Data[t.Name.Local] = value
		case xml.EndElement:
			// Consumed the inner element; skip remaining tokens up to </UserData>.
			return d.Skip()
		}
	}
}

func (u UserData) asMap() map[string]any {
	result := map[string]any{
		"name": u.Name,
	}
	if len(u.Data) > 0 {
		result["data"] = u.Data
	}
	return result
}

// ProcessingErrorData contains error information when an event cannot be rendered.
// https://learn.microsoft.com/en-us/windows/win32/wes/eventschema-processingerrordata-eventtype-element
type ProcessingErrorData struct {
	ErrorCode    uint32 `xml:"ErrorCode"`
	DataItemName string `xml:"DataItemName"`
	EventPayload string `xml:"EventPayload"`
}

func (p ProcessingErrorData) asMap() map[string]any {
	return map[string]any{
		"error_code":     p.ErrorCode,
		"data_item_name": p.DataItemName,
		"event_payload":  p.EventPayload,
	}
}

// DebugData contains data logged for Windows software tracing.
// https://learn.microsoft.com/en-us/windows/win32/wes/eventschema-debugdata-eventtype-element
type DebugData struct {
	SequenceNumber uint32 `xml:"SequenceNumber"`
	FlagName       string `xml:"FlagName"`
	LevelName      string `xml:"LevelName"`
	Component      string `xml:"Component"`
	SubComponent   string `xml:"SubComponent"`
	FileLine       string `xml:"FileLine"`
	Function       string `xml:"Function"`
	Message        string `xml:"Message"`
}

func (d DebugData) asMap() map[string]any {
	return map[string]any{
		"sequence_number": d.SequenceNumber,
		"flag_name":       d.FlagName,
		"level_name":      d.LevelName,
		"component":       d.Component,
		"sub_component":   d.SubComponent,
		"file_line":       d.FileLine,
		"function":        d.Function,
		"message":         d.Message,
	}
}

// parsedEvent is the interface consumed by sendEvent. All fields accessed in
// sendEvent must go through this interface so that the compiler enforces that
// rawParsedEvent explicitly supports any new access added to the raw path.
type parsedEvent interface {
	getOriginal() string
	getSystemTime() string
	getLevel() string
	getRenderedLevel() string
	// formattedBody returns the structured body map for non-raw mode.
	// Panics if called on rawParsedEvent — only valid when raw=false.
	toEventXML() *EventXML
}

func (e *EventXML) getOriginal() string   { return e.Original }
func (e *EventXML) getSystemTime() string { return e.TimeCreated.SystemTime }
func (e *EventXML) getLevel() string      { return e.Level }
func (e *EventXML) getRenderedLevel() string {
	if e.RenderingInfo != nil {
		return e.RenderingInfo.Level
	}
	return ""
}
func (e *EventXML) toEventXML() *EventXML { return e }

// unmarshalEventXML will unmarshal EventXML from xml bytes.
// Illegal XML 1.0 characters (e.g. U+0001 found in some Sysmon events) are
// stripped before parsing so that a single malformed event does not halt the
// entire receiver.
func unmarshalEventXML(data []byte) (parsedEvent, error) {
	sanitized := sanitizeXMLBytes(data)
	var eventXML EventXML
	if err := xml.Unmarshal(sanitized, &eventXML); err != nil {
		return nil, fmt.Errorf("failed to unmarshal xml bytes into event: %w (%s)", err, string(sanitized))
	}
	// The sanitized bytes are only required for XML unmarshalling - the original data is preserved.
	eventXML.Original = string(data)
	return &eventXML, nil
}

// rawEventXML holds only the fields needed when raw=true, avoiding the
// allocation and parsing cost of the full EventXML struct. The RenderingInfo
// field carries just the rendered level, which is used for severity even in
// raw mode when the deep render path is active.
type rawEventXML struct {
	Original      string               `xml:"-"`
	TimeCreated   TimeCreated          `xml:"System>TimeCreated"`
	Level         string               `xml:"System>Level"`
	RenderingInfo *rawRenderingInfoXML `xml:"RenderingInfo"`
}

// rawRenderingInfoXML holds only the Level field from RenderingInfo.
type rawRenderingInfoXML struct {
	Level string `xml:"Level"`
}

func (r *rawEventXML) getOriginal() string   { return r.Original }
func (r *rawEventXML) getSystemTime() string { return r.TimeCreated.SystemTime }
func (r *rawEventXML) getLevel() string      { return r.Level }
func (r *rawEventXML) getRenderedLevel() string {
	if r.RenderingInfo != nil {
		return r.RenderingInfo.Level
	}
	return ""
}

func (*rawEventXML) toEventXML() *EventXML {
	panic("toEventXML called on rawEventXML: only valid in non-raw mode")
}

// unmarshalRawEventXML parses only the fields needed when raw=true and returns
// a rawParsedEvent. Use this instead of unmarshalEventXML when raw=true to
// avoid populating fields that will not be used.
func unmarshalRawEventXML(data []byte) (parsedEvent, error) {
	sanitized := sanitizeXMLBytes(data)
	var raw rawEventXML
	if err := xml.Unmarshal(sanitized, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal xml bytes into event: %w (%s)", err, string(sanitized))
	}
	// The sanitized bytes are only required for XML unmarshalling - the original data is preserved.
	raw.Original = string(data)
	return &raw, nil
}

// sanitizeXMLBytes removes characters that are illegal in XML 1.0 documents.
// XML 1.0 permits: #x9 | #xA | #xD | [#x20-#xD7FF] | [#xE000-#xFFFD] | [#x10000-#x10FFFF]
// All other code points (e.g. U+0001–U+0008, U+000B, U+000C, U+000E–U+001F) are dropped.
//
// A zero-allocation pre-scan is performed first; if no illegal bytes are found
// the original slice is returned unchanged, avoiding the bytes.Map allocation
// on the common (clean) path.
func sanitizeXMLBytes(data []byte) []byte {
	if !hasIllegalXMLBytes(data) {
		return data
	}
	return bytes.Map(func(r rune) rune {
		if (r >= 0x20 && r <= 0xD7FF) || r == 0x09 || r == 0x0A || r == 0x0D ||
			(r >= 0xE000 && r <= 0xFFFD) ||
			r >= 0x10000 {
			return r
		}
		return -1
	}, data)
}

// hasIllegalXMLBytes reports whether data contains any character that is
// illegal in an XML 1.0 document. It operates on raw bytes to avoid
// allocation: single-byte control characters are caught by a simple range
// check, and the only illegal multi-byte sequences handled explicitly are
// U+FFFE (EF BF BE) and U+FFFF (EF BF BF).
func hasIllegalXMLBytes(data []byte) bool {
	for i := range len(data) {
		b := data[i]
		if b < 0x20 {
			// 0x09 (tab), 0x0A (LF), 0x0D (CR) are the only legal control chars.
			if b != 0x09 && b != 0x0A && b != 0x0D {
				return true
			}
		} else if b == 0xEF && i+2 < len(data) {
			// U+FFFE = EF BF BE, U+FFFF = EF BF BF — both illegal in XML 1.0.
			if data[i+1] == 0xBF && (data[i+2] == 0xBE || data[i+2] == 0xBF) {
				return true
			}
		}
	}
	return false
}
