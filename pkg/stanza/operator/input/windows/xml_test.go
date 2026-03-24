// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestParseValidTimestamp(t *testing.T) {
	timestamp := parseTimestamp("2020-07-30T01:01:01.123456789Z")
	expected, _ := time.Parse(time.RFC3339Nano, "2020-07-30T01:01:01.123456789Z")
	require.Equal(t, expected, timestamp)
}

func TestParseInvalidTimestamp(t *testing.T) {
	timestamp := parseTimestamp("invalid")
	require.Equal(t, time.Now().Year(), timestamp.Year())
	require.Equal(t, time.Now().Month(), timestamp.Month())
	require.Equal(t, time.Now().Day(), timestamp.Day())
}

func TestParseSeverity(t *testing.T) {
	require.Equal(t, entry.Fatal, parseSeverity("Critical", ""))
	require.Equal(t, entry.Error, parseSeverity("Error", ""))
	require.Equal(t, entry.Warn, parseSeverity("Warning", ""))
	require.Equal(t, entry.Info, parseSeverity("Information", ""))
	require.Equal(t, entry.Default, parseSeverity("Unknown", ""))
	require.Equal(t, entry.Fatal, parseSeverity("", "1"))
	require.Equal(t, entry.Error, parseSeverity("", "2"))
	require.Equal(t, entry.Warn, parseSeverity("", "3"))
	require.Equal(t, entry.Info, parseSeverity("", "4"))
	require.Equal(t, entry.Default, parseSeverity("", "0"))
}

func TestParseBody(t *testing.T) {
	xml := &EventXML{
		EventID: EventID{
			ID:         1,
			Qualifiers: 2,
		},
		Provider: Provider{
			Name:            "provider",
			GUID:            "guid",
			EventSourceName: "event source",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2020-07-30T01:01:01.123456789Z",
		},
		Computer: "computer",
		Channel:  "application",
		RecordID: 1,
		Level:    "Information",
		Task:     "task",
		Opcode:   "opcode",
		Keywords: []string{"keyword"},
		EventData: EventData{
			Data: []Data{{Name: "1st_name", Value: "value"}, {Name: "2nd_name", Value: "another_value"}},
		},
		RenderingInfo: &RenderingInfo{
			Message:  "message",
			Level:    "rendered_level",
			Task:     "rendered_task",
			Opcode:   "rendered_opcode",
			Keywords: []string{"RenderedKeywords"},
		},
		Version: 0,
	}

	expected := map[string]any{
		"event_id": map[string]any{
			"id":         uint32(1),
			"qualifiers": uint16(2),
		},
		"provider": map[string]any{
			"name":         "provider",
			"guid":         "guid",
			"event_source": "event source",
		},
		"system_time": "2020-07-30T01:01:01.123456789Z",
		"computer":    "computer",
		"channel":     "application",
		"record_id":   uint64(1),
		"level":       "rendered_level",
		"message":     "message",
		"task":        "rendered_task",
		"opcode":      "rendered_opcode",
		"keywords":    []string{"RenderedKeywords"},
		"event_data": map[string]any{
			"1st_name": "value",
			"2nd_name": "another_value",
		},
		"rendering_info": map[string]any{
			"culture":  "",
			"message":  "message",
			"level":    "rendered_level",
			"task":     "rendered_task",
			"opcode":   "rendered_opcode",
			"channel":  "",
			"provider": "",
			"keywords": []string{"RenderedKeywords"},
		},
		"version": uint8(0),
	}

	require.Equal(t, expected, formattedBody(xml, EventDataFormatMap))
}

func TestParseBodySecurityExecution(t *testing.T) {
	xml := &EventXML{
		EventID: EventID{
			ID:         1,
			Qualifiers: 2,
		},
		Provider: Provider{
			Name:            "provider",
			GUID:            "guid",
			EventSourceName: "event source",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2020-07-30T01:01:01.123456789Z",
		},
		Computer: "computer",
		Channel:  "application",
		RecordID: 1,
		Level:    "Information",
		Task:     "task",
		Opcode:   "opcode",
		Keywords: []string{"keyword"},
		EventData: EventData{
			Data: []Data{{Name: "name", Value: "value"}, {Name: "another_name", Value: "another_value"}},
		},
		Execution: &Execution{
			ProcessID: 13,
			ThreadID:  102,
		},
		Security: &Security{
			UserID: "my-user-id",
		},
		RenderingInfo: &RenderingInfo{
			Message:  "message",
			Level:    "rendered_level",
			Task:     "rendered_task",
			Opcode:   "rendered_opcode",
			Keywords: []string{"RenderedKeywords"},
		},
		Version: 0,
	}

	expected := map[string]any{
		"event_id": map[string]any{
			"id":         uint32(1),
			"qualifiers": uint16(2),
		},
		"provider": map[string]any{
			"name":         "provider",
			"guid":         "guid",
			"event_source": "event source",
		},
		"system_time": "2020-07-30T01:01:01.123456789Z",
		"computer":    "computer",
		"channel":     "application",
		"record_id":   uint64(1),
		"level":       "rendered_level",
		"message":     "message",
		"task":        "rendered_task",
		"opcode":      "rendered_opcode",
		"keywords":    []string{"RenderedKeywords"},
		"execution": map[string]any{
			"process_id": uint(13),
			"thread_id":  uint(102),
		},
		"security": map[string]any{
			"user_id": "my-user-id",
		},
		"event_data": map[string]any{
			"name":         "value",
			"another_name": "another_value",
		},
		"rendering_info": map[string]any{
			"culture":  "",
			"message":  "message",
			"level":    "rendered_level",
			"task":     "rendered_task",
			"opcode":   "rendered_opcode",
			"channel":  "",
			"provider": "",
			"keywords": []string{"RenderedKeywords"},
		},
		"version": uint8(0),
	}

	require.Equal(t, expected, formattedBody(xml, EventDataFormatMap))
}

func TestParseBodyFullExecution(t *testing.T) {
	processorID := uint(3)
	sessionID := uint(2)
	kernelTime := uint(3)
	userTime := uint(100)
	processorTime := uint(200)

	xml := &EventXML{
		EventID: EventID{
			ID:         1,
			Qualifiers: 2,
		},
		Provider: Provider{
			Name:            "provider",
			GUID:            "guid",
			EventSourceName: "event source",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2020-07-30T01:01:01.123456789Z",
		},
		Computer: "computer",
		Channel:  "application",
		RecordID: 1,
		Level:    "Information",
		Task:     "task",
		Opcode:   "opcode",
		Keywords: []string{"keyword"},
		EventData: EventData{
			Data: []Data{{Name: "name", Value: "value"}, {Name: "another_name", Value: "another_value"}},
		},
		Execution: &Execution{
			ProcessID:     13,
			ThreadID:      102,
			ProcessorID:   &processorID,
			SessionID:     &sessionID,
			KernelTime:    &kernelTime,
			UserTime:      &userTime,
			ProcessorTime: &processorTime,
		},
		Security: &Security{
			UserID: "my-user-id",
		},
		RenderingInfo: &RenderingInfo{
			Message:  "message",
			Level:    "rendered_level",
			Task:     "rendered_task",
			Opcode:   "rendered_opcode",
			Keywords: []string{"RenderedKeywords"},
		},
		Version: 0,
	}

	expected := map[string]any{
		"event_id": map[string]any{
			"id":         uint32(1),
			"qualifiers": uint16(2),
		},
		"provider": map[string]any{
			"name":         "provider",
			"guid":         "guid",
			"event_source": "event source",
		},
		"system_time": "2020-07-30T01:01:01.123456789Z",
		"computer":    "computer",
		"channel":     "application",
		"record_id":   uint64(1),
		"level":       "rendered_level",
		"message":     "message",
		"task":        "rendered_task",
		"opcode":      "rendered_opcode",
		"keywords":    []string{"RenderedKeywords"},
		"execution": map[string]any{
			"process_id":     uint(13),
			"thread_id":      uint(102),
			"processor_id":   processorID,
			"session_id":     sessionID,
			"kernel_time":    kernelTime,
			"user_time":      userTime,
			"processor_time": processorTime,
		},
		"security": map[string]any{
			"user_id": "my-user-id",
		},
		"event_data": map[string]any{
			"name":         "value",
			"another_name": "another_value",
		},
		"rendering_info": map[string]any{
			"culture":  "",
			"message":  "message",
			"level":    "rendered_level",
			"task":     "rendered_task",
			"opcode":   "rendered_opcode",
			"channel":  "",
			"provider": "",
			"keywords": []string{"RenderedKeywords"},
		},
		"version": uint8(0),
	}

	require.Equal(t, expected, formattedBody(xml, EventDataFormatMap))
}

func TestParseBodyCorrelation(t *testing.T) {
	activityIDGuid := "{11111111-1111-1111-1111-111111111111}"
	relatedActivityIDGuid := "{22222222-2222-2222-2222-222222222222}"
	xml := &EventXML{
		EventID: EventID{
			ID:         1,
			Qualifiers: 2,
		},
		Provider: Provider{
			Name:            "provider",
			GUID:            "guid",
			EventSourceName: "event source",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2020-07-30T01:01:01.123456789Z",
		},
		Computer: "computer",
		Channel:  "application",
		RecordID: 1,
		Level:    "Information",
		Task:     "task",
		Opcode:   "opcode",
		Keywords: []string{"keyword"},
		EventData: EventData{
			Data: []Data{{Name: "1st_name", Value: "value"}, {Name: "2nd_name", Value: "another_value"}},
		},
		RenderingInfo: &RenderingInfo{
			Message:  "message",
			Level:    "rendered_level",
			Task:     "rendered_task",
			Opcode:   "rendered_opcode",
			Keywords: []string{"RenderedKeywords"},
		},
		Correlation: &Correlation{
			ActivityID:        &activityIDGuid,
			RelatedActivityID: &relatedActivityIDGuid,
		},
		Version: 1,
	}

	expected := map[string]any{
		"event_id": map[string]any{
			"id":         uint32(1),
			"qualifiers": uint16(2),
		},
		"provider": map[string]any{
			"name":         "provider",
			"guid":         "guid",
			"event_source": "event source",
		},
		"system_time": "2020-07-30T01:01:01.123456789Z",
		"computer":    "computer",
		"channel":     "application",
		"record_id":   uint64(1),
		"level":       "rendered_level",
		"message":     "message",
		"task":        "rendered_task",
		"opcode":      "rendered_opcode",
		"keywords":    []string{"RenderedKeywords"},
		"event_data": map[string]any{
			"1st_name": "value",
			"2nd_name": "another_value",
		},
		"rendering_info": map[string]any{
			"culture":  "",
			"message":  "message",
			"level":    "rendered_level",
			"task":     "rendered_task",
			"opcode":   "rendered_opcode",
			"channel":  "",
			"provider": "",
			"keywords": []string{"RenderedKeywords"},
		},
		"correlation": map[string]any{
			"activity_id":         "{11111111-1111-1111-1111-111111111111}",
			"related_activity_id": "{22222222-2222-2222-2222-222222222222}",
		},
		"version": uint8(1),
	}

	require.Equal(t, expected, formattedBody(xml, EventDataFormatMap))
}

func TestParseNoRendered(t *testing.T) {
	xml := &EventXML{
		EventID: EventID{
			ID:         1,
			Qualifiers: 2,
		},
		Provider: Provider{
			Name:            "provider",
			GUID:            "guid",
			EventSourceName: "event source",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2020-07-30T01:01:01.123456789Z",
		},
		Computer: "computer",
		Channel:  "application",
		RecordID: 1,
		Level:    "Information",
		Task:     "task",
		Opcode:   "opcode",
		Keywords: []string{"keyword"},
		EventData: EventData{
			Data: []Data{{Name: "name", Value: "value"}, {Name: "another_name", Value: "another_value"}},
		},
		Version: 0,
	}

	expected := map[string]any{
		"event_id": map[string]any{
			"id":         uint32(1),
			"qualifiers": uint16(2),
		},
		"provider": map[string]any{
			"name":         "provider",
			"guid":         "guid",
			"event_source": "event source",
		},
		"system_time": "2020-07-30T01:01:01.123456789Z",
		"computer":    "computer",
		"channel":     "application",
		"record_id":   uint64(1),
		"level":       "Information",
		"message":     "",
		"task":        "task",
		"opcode":      "opcode",
		"keywords":    []string{"keyword"},
		"event_data": map[string]any{
			"name":         "value",
			"another_name": "another_value",
		},
		"version": uint8(0),
	}

	require.Equal(t, expected, formattedBody(xml, EventDataFormatMap))
}

func TestParseBodySecurity(t *testing.T) {
	xml := &EventXML{
		EventID: EventID{
			ID:         1,
			Qualifiers: 2,
		},
		Provider: Provider{
			Name:            "provider",
			GUID:            "guid",
			EventSourceName: "event source",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2020-07-30T01:01:01.123456789Z",
		},
		Computer: "computer",
		Channel:  "Security",
		RecordID: 1,
		Level:    "Information",
		Task:     "task",
		Opcode:   "opcode",
		Keywords: []string{"keyword"},
		EventData: EventData{
			Data: []Data{{Name: "name", Value: "value"}, {Name: "another_name", Value: "another_value"}},
		},
		RenderingInfo: &RenderingInfo{
			Message:  "message",
			Level:    "rendered_level",
			Task:     "rendered_task",
			Opcode:   "rendered_opcode",
			Keywords: []string{"RenderedKeywords"},
		},
		Version: 0,
	}

	expected := map[string]any{
		"event_id": map[string]any{
			"id":         uint32(1),
			"qualifiers": uint16(2),
		},
		"provider": map[string]any{
			"name":         "provider",
			"guid":         "guid",
			"event_source": "event source",
		},
		"system_time": "2020-07-30T01:01:01.123456789Z",
		"computer":    "computer",
		"channel":     "Security",
		"record_id":   uint64(1),
		"level":       "rendered_level",
		"message":     "message",
		"task":        "rendered_task",
		"opcode":      "rendered_opcode",
		"keywords":    []string{"RenderedKeywords"},
		"event_data": map[string]any{
			"name":         "value",
			"another_name": "another_value",
		},
		"rendering_info": map[string]any{
			"culture":  "",
			"message":  "message",
			"level":    "rendered_level",
			"task":     "rendered_task",
			"opcode":   "rendered_opcode",
			"channel":  "",
			"provider": "",
			"keywords": []string{"RenderedKeywords"},
		},
		"version": uint8(0),
	}

	require.Equal(t, expected, formattedBody(xml, EventDataFormatMap))
}

func TestParseEventData(t *testing.T) {
	xmlMap := &EventXML{
		EventData: EventData{
			Name:   "EVENT_DATA",
			Data:   []Data{{Name: "field", Value: "value"}},
			Binary: "2D20",
		},
	}

	parsed := formattedBody(xmlMap, EventDataFormatMap)
	expectedMap := map[string]any{
		"name":   "EVENT_DATA",
		"field":  "value",
		"binary": "2D20",
	}
	require.Equal(t, expectedMap, parsed["event_data"])

	xmlMixed := &EventXML{
		EventData: EventData{
			Data: []Data{{Name: "named_field", Value: "value"}, {Value: "no_name"}},
		},
	}

	parsed = formattedBody(xmlMixed, EventDataFormatMap)
	expectedFlat := map[string]any{
		"named_field": "value",
		"param1":      "no_name",
	}
	require.Equal(t, expectedFlat, parsed["event_data"])
}

func TestParseBodyWithUserData(t *testing.T) {
	e := &EventXML{
		Channel: "Security",
		UserData: &UserData{
			Name: "LogFileCleared",
			Data: map[string]string{
				"SubjectUserName": "test_user",
				"SubjectLogonId":  "0xa8bb72",
			},
		},
	}

	body := formattedBody(e, EventDataFormatArray)
	require.Equal(t, map[string]any{
		"name": "LogFileCleared",
		"data": map[string]string{
			"SubjectUserName": "test_user",
			"SubjectLogonId":  "0xa8bb72",
		},
	}, body["user_data"])
}

// TestUnmarshalSystemEventWithRenderingInfo uses a real captured System channel event
// (Service Control Manager 7036). It confirms that Culture and Provider are populated
// from RenderingInfo, while Channel/Task/Opcode are absent in RenderingInfo for this
// event type and fall back to the System values in the body.
func TestUnmarshalSystemEventWithRenderingInfo(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xmlRenderingInfoSystem.xml"))
	require.NoError(t, err)

	event, err := unmarshalEventXML(data)
	require.NoError(t, err)

	require.NotNil(t, event.RenderingInfo)
	require.Equal(t, "en-US", event.RenderingInfo.Culture)
	require.Equal(t, "The Software Protection service entered the running state.", event.RenderingInfo.Message)
	require.Equal(t, "Information", event.RenderingInfo.Level)
	require.Equal(t, "Microsoft-Windows-Service Control Manager", event.RenderingInfo.Provider)
	require.Equal(t, []string{"Classic"}, event.RenderingInfo.Keywords)
	// Channel, Task, Opcode are absent from RenderingInfo in this event type.
	require.Empty(t, event.RenderingInfo.Channel)
	require.Empty(t, event.RenderingInfo.Task)
	require.Empty(t, event.RenderingInfo.Opcode)

	body := formattedBody(event, EventDataFormatArray)

	// Level, Task, Opcode fall back to System values when absent from RenderingInfo.
	require.Equal(t, "Information", body["level"])
	require.Equal(t, "0", body["task"])
	require.Equal(t, "0", body["opcode"])

	ri, ok := body["rendering_info"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "en-US", ri["culture"])
	require.Equal(t, "Microsoft-Windows-Service Control Manager", ri["provider"])
	require.Empty(t, ri["channel"])
}

// TestUnmarshalSecurityEventWithRenderingInfo uses a real captured Security channel event
// (4624 logon). It confirms that all RenderingInfo fields — including Channel, Provider,
// Task, Opcode, and Culture — are populated when a well-known provider manifest is present.
func TestUnmarshalSecurityEventWithRenderingInfo(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xmlRenderingInfoSecurity.xml"))
	require.NoError(t, err)

	event, err := unmarshalEventXML(data)
	require.NoError(t, err)

	require.NotNil(t, event.RenderingInfo)
	require.Equal(t, "en-US", event.RenderingInfo.Culture)
	require.Equal(t, "Information", event.RenderingInfo.Level)
	require.Equal(t, "Logon", event.RenderingInfo.Task)
	require.Equal(t, "Info", event.RenderingInfo.Opcode)
	require.Equal(t, "Security", event.RenderingInfo.Channel)
	require.Equal(t, "Microsoft Windows security auditing.", event.RenderingInfo.Provider)
	require.Equal(t, []string{"Audit Success"}, event.RenderingInfo.Keywords)
	require.Contains(t, event.RenderingInfo.Message, "An account was successfully logged on.")

	body := formattedBody(event, EventDataFormatArray)

	// Rendered values from RenderingInfo take precedence over raw System values.
	require.Equal(t, "Information", body["level"]) // vs raw "0"
	require.Equal(t, "Logon", body["task"])        // vs raw "12544"
	require.Equal(t, "Info", body["opcode"])       // vs raw "0"

	ri, ok := body["rendering_info"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "en-US", ri["culture"])
	require.Equal(t, "Security", ri["channel"])
	require.Equal(t, "Microsoft Windows security auditing.", ri["provider"])
}

func TestInvalidUnmarshal(t *testing.T) {
	_, err := unmarshalEventXML([]byte("Test \n Invalid \t Unmarshal"))
	require.Error(t, err)
}

func TestSanitizeXMLBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "no illegal chars",
			input:    []byte("hello world"),
			expected: []byte("hello world"),
		},
		{
			name:     "tab newline carriage return are kept",
			input:    []byte("a\tb\nc\rd"),
			expected: []byte("a\tb\nc\rd"),
		},
		{
			name:     "U+0001 is stripped",
			input:    []byte("before\x01after"),
			expected: []byte("beforeafter"),
		},
		{
			name:     "multiple control chars are stripped",
			input:    []byte("\x01\x02\x03\x04\x05\x06\x07\x08\x0B\x0C\x0E\x0F\x10\x1F"),
			expected: []byte(""),
		},
		{
			name:     "U+FFFE and U+FFFF are stripped",
			input:    []byte{0xEF, 0xBF, 0xBE, 0xEF, 0xBF, 0xBF}, // U+FFFE and U+FFFF in UTF-8
			expected: []byte(""),
		},
		{
			name:     "valid non-ASCII chars are kept",
			input:    []byte("caf\xC3\xA9"), // "café" in UTF-8
			expected: []byte("caf\xC3\xA9"),
		},
		{
			name:     "empty input",
			input:    []byte{},
			expected: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeXMLBytes(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestUnmarshalWithIllegalXMLChars verifies that events containing characters
// illegal in XML 1.0 (such as U+0001, observed in Sysmon Operational events)
// are sanitized and parsed successfully rather than causing an error.
func TestUnmarshalWithIllegalXMLChars(t *testing.T) {
	// Build XML that contains U+0001 inside a Data element, as seen in Sysmon events
	// where file metadata contains embedded control characters.
	xmlTemplate := `<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">
    <System>
        <Provider Name="Microsoft-Windows-Sysmon" Guid="{5770385F-C22A-43E0-BF4C-06F5698FFBD9}" />
        <EventID Qualifiers="0">1</EventID>
        <Version>5</Version>
        <Level>4</Level>
        <Task>1</Task>
        <Opcode>0</Opcode>
        <Keywords>0x8000000000000000</Keywords>
        <TimeCreated SystemTime="2023-10-19T21:57:58.0685414Z" />
        <EventRecordID>12345</EventRecordID>
        <Correlation />
        <Execution ProcessID="1234" ThreadID="5678" />
        <Channel>Microsoft-Windows-Sysmon/Operational</Channel>
        <Computer>computer</Computer>
        <Security />
    </System>
    <EventData>
        <Data Name="FileVersion">` + "1.0\x01.0" + `</Data>
    </EventData>
</Event>`

	event, err := unmarshalEventXML([]byte(xmlTemplate))
	require.NoError(t, err)
	require.Equal(t, "Microsoft-Windows-Sysmon/Operational", event.Channel)
	require.Equal(t, uint64(12345), event.RecordID)
	// The illegal character should have been stripped from the event data value
	require.Len(t, event.EventData.Data, 1)
	require.Equal(t, "FileVersion", event.EventData.Data[0].Name)
	require.Equal(t, "1.0.0", event.EventData.Data[0].Value)
	// Original preserves the raw input verbatim, so the illegal character is still present.
	require.Contains(t, event.Original, "\x01")
}

func TestUnmarshalWithEventData(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xmlSample.xml"))
	require.NoError(t, err)

	event, err := unmarshalEventXML(data)
	require.NoError(t, err)

	xml := &EventXML{
		EventID: EventID{
			ID:         16384,
			Qualifiers: 16384,
		},
		Provider: Provider{
			Name:            "Microsoft-Windows-Security-SPP",
			GUID:            "{E23B33B0-C8C9-472C-A5F9-F2BDFEA0F156}",
			EventSourceName: "Software Protection Platform Service",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2022-04-22T10:20:52.3778625Z",
		},
		Computer:  "computer",
		Channel:   "Application",
		RecordID:  23401,
		Level:     "4",
		Task:      "0",
		Opcode:    "0",
		Execution: &Execution{},
		Security:  &Security{},
		EventData: EventData{
			Data: []Data{
				{Name: "Time", Value: "2022-04-28T19:48:52Z"},
				{Name: "Source", Value: "RulesEngine"},
			},
		},
		Keywords:    []string{"0x80000000000000"},
		Original:    string(data),
		Correlation: &Correlation{},
	}

	require.Equal(t, xml, event)
}

func TestUnmarshalWithCorrelation(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xmlWithCorrelation.xml"))
	require.NoError(t, err)

	event, err := unmarshalEventXML(data)
	require.NoError(t, err)

	activityIDGuid := "{11111111-1111-1111-1111-111111111111}"
	relatedActivityIDGuid := "{22222222-2222-2222-2222-222222222222}"

	xml := &EventXML{
		EventID: EventID{
			ID:         4624,
			Qualifiers: 0,
		},
		Provider: Provider{
			Name: "Microsoft-Windows-Security-Auditing",
			GUID: "{54849625-5478-4994-a5ba-3e3b0328c30d}",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2025-12-02T23:33:05.2167526Z",
		},
		Computer: "computer",
		Channel:  "Security",
		RecordID: 13177,
		Level:    "0",
		Task:     "12544",
		Opcode:   "0",
		EventData: EventData{
			Data:   []Data{{Name: "SubjectDomainName", Value: "WORKGROUP"}},
			Binary: "",
		},
		Keywords: []string{"0x8020000000000000"},
		Security: &Security{},
		Execution: &Execution{
			ProcessID: 800,
			ThreadID:  7852,
		},
		Original: string(data),
		Correlation: &Correlation{
			ActivityID:        &activityIDGuid,
			RelatedActivityID: &relatedActivityIDGuid,
		},
		Version: 2,
	}

	require.Equal(t, xml, event)
}

func TestUnmarshalWithAnonymousEventDataEntries(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xmlWithAnonymousEventDataEntries.xml"))
	require.NoError(t, err)

	event, err := unmarshalEventXML(data)
	require.NoError(t, err)

	xml := &EventXML{
		EventID: EventID{
			ID:         8194,
			Qualifiers: 0,
		},
		Provider: Provider{
			Name: "VSS",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2023-10-19T21:57:58.0685414Z",
		},
		Computer: "computer",
		Channel:  "Application",
		RecordID: 383972,
		Level:    "2",
		Task:     "0",
		Opcode:   "0",
		EventData: EventData{
			Data:   []Data{{Name: "", Value: "1st_value"}, {Name: "", Value: "2nd_value"}},
			Binary: "2D20",
		},
		Keywords:    []string{"0x80000000000000"},
		Security:    &Security{},
		Execution:   &Execution{},
		Original:    string(data),
		Correlation: &Correlation{},
		Version:     0,
	}

	require.Equal(t, xml, event)
}

// Benchmarks for unmarshalEventXML measure the cost of the sanitization path
// relative to the baseline (no sanitization) so we can quantify the overhead
// introduced by the illegal-XML-character fix.

// benchmarkXMLClean is a representative Windows event log XML document with no
// illegal characters.  It is intentionally inline so the benchmark does not
// vary with testdata file I/O.
var benchmarkXMLClean = []byte(`<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">
    <System>
        <Provider Name="Microsoft-Windows-Sysmon" Guid="{5770385F-C22A-43E0-BF4C-06F5698FFBD9}" />
        <EventID Qualifiers="0">1</EventID>
        <Version>5</Version>
        <Level>4</Level>
        <Task>1</Task>
        <Opcode>0</Opcode>
        <Keywords>0x8000000000000000</Keywords>
        <TimeCreated SystemTime="2023-10-19T21:57:58.0685414Z" />
        <EventRecordID>12345</EventRecordID>
        <Correlation />
        <Execution ProcessID="1234" ThreadID="5678" />
        <Channel>Microsoft-Windows-Sysmon/Operational</Channel>
        <Computer>computer</Computer>
        <Security />
    </System>
    <EventData>
        <Data Name="FileVersion">1.0.0</Data>
        <Data Name="Description">A perfectly normal file</Data>
        <Data Name="Product">Windows</Data>
    </EventData>
</Event>`)

// benchmarkXMLDirty is the same document but with an embedded U+0001 control
// character (as observed in real Sysmon events) that is illegal in XML 1.0.
var benchmarkXMLDirty = []byte("<Event xmlns=\"http://schemas.microsoft.com/win/2004/08/events/event\">\n" +
	"    <System>\n" +
	"        <Provider Name=\"Microsoft-Windows-Sysmon\" Guid=\"{5770385F-C22A-43E0-BF4C-06F5698FFBD9}\" />\n" +
	"        <EventID Qualifiers=\"0\">1</EventID>\n" +
	"        <Version>5</Version>\n" +
	"        <Level>4</Level>\n" +
	"        <Task>1</Task>\n" +
	"        <Opcode>0</Opcode>\n" +
	"        <Keywords>0x8000000000000000</Keywords>\n" +
	"        <TimeCreated SystemTime=\"2023-10-19T21:57:58.0685414Z\" />\n" +
	"        <EventRecordID>12345</EventRecordID>\n" +
	"        <Correlation />\n" +
	"        <Execution ProcessID=\"1234\" ThreadID=\"5678\" />\n" +
	"        <Channel>Microsoft-Windows-Sysmon/Operational</Channel>\n" +
	"        <Computer>computer</Computer>\n" +
	"        <Security />\n" +
	"    </System>\n" +
	"    <EventData>\n" +
	"        <Data Name=\"FileVersion\">1.0\x01.0</Data>\n" +
	"        <Data Name=\"Description\">A file with an \x02illegal\x03 char</Data>\n" +
	"    </EventData>\n" +
	"</Event>")

// BenchmarkXMLUnmarshal_Baseline calls xml.Unmarshal directly with no
// sanitization, giving the true lower bound to compare against.
func BenchmarkXMLUnmarshal_Baseline(b *testing.B) {
	for b.Loop() {
		var e EventXML
		_ = xml.Unmarshal(benchmarkXMLClean, &e)
	}
}

// BenchmarkUnmarshalEventXML_CleanInput measures the common-case cost of
// unmarshalling a typical event with no illegal XML characters. The pre-scan
// short-circuits before any allocation, so overhead above the baseline should
// be minimal.
func BenchmarkUnmarshalEventXML_CleanInput(b *testing.B) {
	for b.Loop() {
		_, _ = unmarshalEventXML(benchmarkXMLClean)
	}
}

// BenchmarkUnmarshalEventXML_DirtyInput measures the cost when the input
// contains illegal XML 1.0 characters (as seen in some Sysmon events).
func BenchmarkUnmarshalEventXML_DirtyInput(b *testing.B) {
	for b.Loop() {
		_, _ = unmarshalEventXML(benchmarkXMLDirty)
	}
}

// BenchmarkSanitizeXMLBytes_CleanInput isolates the cost of the sanitization
// step on typical clean input, making it easy to compare against the full
// unmarshal cost above.
func BenchmarkSanitizeXMLBytes_CleanInput(b *testing.B) {
	for b.Loop() {
		_ = sanitizeXMLBytes(benchmarkXMLClean)
	}
}

// BenchmarkSanitizeXMLBytes_DirtyInput isolates the cost of the sanitization
// step when the input contains illegal characters.
func BenchmarkSanitizeXMLBytes_DirtyInput(b *testing.B) {
	for b.Loop() {
		_ = sanitizeXMLBytes(benchmarkXMLDirty)
	}
}

func TestUnmarshalWithUserData(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xmlSampleUserData.xml"))
	require.NoError(t, err)

	event, err := unmarshalEventXML(data)
	require.NoError(t, err)

	xml := &EventXML{
		EventID: EventID{
			ID: 1102,
		},
		Provider: Provider{
			Name: "Microsoft-Windows-Eventlog",
			GUID: "{fc65ddd8-d6ef-4962-83d5-6e5cfe9ce148}",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2023-10-12T10:38:24.543506200Z",
		},
		Computer: "test.example.com",
		Channel:  "Security",
		RecordID: 2590526,
		Level:    "4",
		Task:     "104",
		Opcode:   "0",
		Keywords: []string{"0x4020000000000000"},
		Security: &Security{
			UserID: "S-1-5-18",
		},
		Execution: &Execution{
			ProcessID: 1472,
			ThreadID:  7784,
		},
		UserData: &UserData{
			Name: "LogFileCleared",
			Data: map[string]string{
				"SubjectUserSid":        "S-1-5-21-1148437859-4135665037-1195073887-1000",
				"SubjectUserName":       "test_user",
				"SubjectDomainName":     "TEST",
				"SubjectLogonId":        "0xa8bb72",
				"ClientProcessId":       "4536",
				"ClientProcessStartKey": "17732923532772643",
			},
		},
		Original:    string(data),
		Correlation: &Correlation{},
		Version:     1,
	}

	require.Equal(t, xml, event)
}

func TestParseBodyWithProcessingErrorData(t *testing.T) {
	e := &EventXML{
		ProcessingErrorData: &ProcessingErrorData{
			ErrorCode:    15005,
			DataItemName: "SubjectUserSid",
			EventPayload: "AABBCCDD",
		},
	}

	body := formattedBody(e, EventDataFormatArray)
	require.Equal(t, map[string]any{
		"error_code":     uint32(15005),
		"data_item_name": "SubjectUserSid",
		"event_payload":  "AABBCCDD",
	}, body["processing_error_data"])
}

func TestUnmarshalWithProcessingErrorData(t *testing.T) {
	xmlStr := `<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">
	<System>
		<Provider Name="TestProvider"/>
		<EventID>1</EventID><Version>0</Version><Level>2</Level><Task>0</Task><Opcode>0</Opcode>
		<Keywords>0x80000000000000</Keywords>
		<TimeCreated SystemTime="2026-01-01T00:00:00.000000000Z"/>
		<EventRecordID>1</EventRecordID>
		<Correlation/><Execution ProcessID="100" ThreadID="200"/>
		<Channel>Application</Channel><Computer>computer</Computer><Security/>
	</System>
	<ProcessingErrorData>
		<ErrorCode>15005</ErrorCode>
		<DataItemName>SubjectUserSid</DataItemName>
		<EventPayload>AABBCCDD</EventPayload>
	</ProcessingErrorData>
</Event>`

	event, err := unmarshalEventXML([]byte(xmlStr))
	require.NoError(t, err)
	require.NotNil(t, event.ProcessingErrorData)
	require.Equal(t, uint32(15005), event.ProcessingErrorData.ErrorCode)
	require.Equal(t, "SubjectUserSid", event.ProcessingErrorData.DataItemName)
	require.Equal(t, "AABBCCDD", event.ProcessingErrorData.EventPayload)
}

func TestParseBodyWithDebugData(t *testing.T) {
	e := &EventXML{
		DebugData: &DebugData{
			SequenceNumber: 42,
			FlagName:       "TRACE_LEVEL_INFORMATION",
			LevelName:      "Information",
			Component:      "TestComponent",
			SubComponent:   "TestSubComponent",
			FileLine:       "main.c:100",
			Function:       "TestFunction",
			Message:        "Debug message",
		},
	}

	body := formattedBody(e, EventDataFormatArray)
	require.Equal(t, map[string]any{
		"sequence_number": uint32(42),
		"flag_name":       "TRACE_LEVEL_INFORMATION",
		"level_name":      "Information",
		"component":       "TestComponent",
		"sub_component":   "TestSubComponent",
		"file_line":       "main.c:100",
		"function":        "TestFunction",
		"message":         "Debug message",
	}, body["debug_data"])
}

func TestUnmarshalWithDebugData(t *testing.T) {
	xmlStr := `<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">
	<System>
		<Provider Name="TestProvider"/>
		<EventID>1</EventID><Version>0</Version><Level>4</Level><Task>0</Task><Opcode>0</Opcode>
		<Keywords>0x80000000000000</Keywords>
		<TimeCreated SystemTime="2026-01-01T00:00:00.000000000Z"/>
		<EventRecordID>2</EventRecordID>
		<Correlation/><Execution ProcessID="100" ThreadID="200"/>
		<Channel>Application</Channel><Computer>computer</Computer><Security/>
	</System>
	<DebugData>
		<SequenceNumber>42</SequenceNumber>
		<FlagName>TRACE_LEVEL_INFORMATION</FlagName>
		<LevelName>Information</LevelName>
		<Component>TestComponent</Component>
		<SubComponent>TestSubComponent</SubComponent>
		<FileLine>main.c:100</FileLine>
		<Function>TestFunction</Function>
		<Message>Debug message</Message>
	</DebugData>
</Event>`

	event, err := unmarshalEventXML([]byte(xmlStr))
	require.NoError(t, err)
	require.NotNil(t, event.DebugData)
	require.Equal(t, uint32(42), event.DebugData.SequenceNumber)
	require.Equal(t, "TRACE_LEVEL_INFORMATION", event.DebugData.FlagName)
	require.Equal(t, "Information", event.DebugData.LevelName)
	require.Equal(t, "TestComponent", event.DebugData.Component)
	require.Equal(t, "TestSubComponent", event.DebugData.SubComponent)
	require.Equal(t, "main.c:100", event.DebugData.FileLine)
	require.Equal(t, "TestFunction", event.DebugData.Function)
	require.Equal(t, "Debug message", event.DebugData.Message)
}

func TestParseBodyWithBinaryEventData(t *testing.T) {
	e := &EventXML{
		BinaryEventData: "AABBCCDD",
	}

	body := formattedBody(e, EventDataFormatArray)
	require.Equal(t, "AABBCCDD", body["binary_event_data"])
}

func TestUnmarshalWithBinaryEventData(t *testing.T) {
	xmlStr := `<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">
	<System>
		<Provider Name="TestProvider"/>
		<EventID>1</EventID><Version>0</Version><Level>2</Level><Task>0</Task><Opcode>0</Opcode>
		<Keywords>0x80000000000000</Keywords>
		<TimeCreated SystemTime="2026-01-01T00:00:00.000000000Z"/>
		<EventRecordID>3</EventRecordID>
		<Correlation/><Execution ProcessID="100" ThreadID="200"/>
		<Channel>Application</Channel><Computer>computer</Computer><Security/>
	</System>
	<BinaryEventData>AABBCCDD</BinaryEventData>
</Event>`

	event, err := unmarshalEventXML([]byte(xmlStr))
	require.NoError(t, err)
	require.Equal(t, "AABBCCDD", event.BinaryEventData)
}

func TestParseEventDataVariants(t *testing.T) {
	tests := []struct {
		name     string
		input    EventData
		expected map[string]any
	}{
		{
			name: "all named",
			input: EventData{
				Data: []Data{
					{Name: "ProcessId", Value: "7924"},
					{Name: "Application", Value: "app.exe"},
				},
			},
			expected: map[string]any{
				"ProcessId":   "7924",
				"Application": "app.exe",
			},
		},
		{
			name: "all anonymous",
			input: EventData{
				Data: []Data{
					{Value: "first"},
					{Value: "second"},
				},
			},
			expected: map[string]any{
				"param1": "first",
				"param2": "second",
			},
		},
		{
			name: "mixed named and anonymous",
			input: EventData{
				Data: []Data{
					{Name: "Named1", Value: "value1"},
					{Value: "anonymous1"},
					{Name: "Named2", Value: "value2"},
					{Value: "anonymous2"},
				},
			},
			expected: map[string]any{
				"Named1": "value1",
				"param1": "anonymous1",
				"Named2": "value2",
				"param2": "anonymous2",
			},
		},
		{
			name: "with name and binary attributes",
			input: EventData{
				Name:   "EVENT_DATA",
				Binary: "2D20",
				Data: []Data{
					{Name: "Field", Value: "value"},
				},
			},
			expected: map[string]any{
				"name":   "EVENT_DATA",
				"binary": "2D20",
				"Field":  "value",
			},
		},
		{
			name:     "empty event data",
			input:    EventData{},
			expected: map[string]any{},
		},
		{
			name: "duplicate named keys - last wins",
			input: EventData{
				Data: []Data{
					{Name: "Key", Value: "first"},
					{Name: "Key", Value: "second"},
				},
			},
			expected: map[string]any{
				"Key": "second",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseEventData(tt.input, EventDataFormatMap)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestParseEventDataArrayFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    EventData
		expected map[string]any
	}{
		{
			name: "named data as array",
			input: EventData{
				Data: []Data{
					{Name: "ProcessId", Value: "7924"},
					{Name: "Application", Value: "app.exe"},
				},
			},
			expected: map[string]any{
				"data": []any{
					map[string]any{"ProcessId": "7924"},
					map[string]any{"Application": "app.exe"},
				},
			},
		},
		{
			name: "anonymous data as array",
			input: EventData{
				Data: []Data{
					{Value: "first"},
					{Value: "second"},
				},
			},
			expected: map[string]any{
				"data": []any{
					map[string]any{"": "first"},
					map[string]any{"": "second"},
				},
			},
		},
		{
			name: "with name and binary attributes",
			input: EventData{
				Name:   "EVENT_DATA",
				Binary: "2D20",
				Data: []Data{
					{Name: "Field", Value: "value"},
				},
			},
			expected: map[string]any{
				"name":   "EVENT_DATA",
				"binary": "2D20",
				"data": []any{
					map[string]any{"Field": "value"},
				},
			},
		},
		{
			name:     "empty event data",
			input:    EventData{},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseEventData(tt.input, EventDataFormatArray)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestParseBodyWithAnonymousEventData(t *testing.T) {
	xml := &EventXML{
		EventID: EventID{
			ID:         1,
			Qualifiers: 2,
		},
		Provider: Provider{
			Name: "provider",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2020-07-30T01:01:01.123456789Z",
		},
		Computer: "computer",
		Channel:  "application",
		RecordID: 1,
		Level:    "Information",
		Task:     "task",
		Opcode:   "opcode",
		Keywords: []string{"keyword"},
		EventData: EventData{
			Data:   []Data{{Value: "first_value"}, {Value: "second_value"}},
			Binary: "2D20",
		},
		RenderingInfo: &RenderingInfo{
			Message: "message",
		},
		Version: 0,
	}

	body := formattedBody(xml, EventDataFormatMap)
	eventData := body["event_data"].(map[string]any)

	require.Equal(t, "first_value", eventData["param1"])
	require.Equal(t, "second_value", eventData["param2"])
	require.Equal(t, "2D20", eventData["binary"])
}

func TestFormattedBodyArrayFormat(t *testing.T) {
	xml := &EventXML{
		EventID: EventID{
			ID:         1,
			Qualifiers: 2,
		},
		Provider: Provider{
			Name:            "provider",
			GUID:            "guid",
			EventSourceName: "event source",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2020-07-30T01:01:01.123456789Z",
		},
		Computer: "computer",
		Channel:  "application",
		RecordID: 1,
		Level:    "Information",
		Task:     "task",
		Opcode:   "opcode",
		Keywords: []string{"keyword"},
		EventData: EventData{
			Data: []Data{{Name: "ProcessId", Value: "7924"}, {Name: "Application", Value: "app.exe"}},
		},
		RenderingInfo: &RenderingInfo{
			Message:  "message",
			Level:    "rendered_level",
			Task:     "rendered_task",
			Opcode:   "rendered_opcode",
			Keywords: []string{"RenderedKeywords"},
		},
		Version: 0,
	}

	body := formattedBody(xml, EventDataFormatArray)
	eventData := body["event_data"].(map[string]any)

	expected := []any{
		map[string]any{"ProcessId": "7924"},
		map[string]any{"Application": "app.exe"},
	}
	require.Equal(t, expected, eventData["data"])
}

func TestUnmarshalAndFormatAnonymousEventData(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xmlWithAnonymousEventDataEntries.xml"))
	require.NoError(t, err)

	event, err := unmarshalEventXML(data)
	require.NoError(t, err)

	mapBody := formattedBody(event, EventDataFormatMap)
	mapEventData := mapBody["event_data"].(map[string]any)
	require.Equal(t, "1st_value", mapEventData["param1"])
	require.Equal(t, "2nd_value", mapEventData["param2"])
	require.Equal(t, "2D20", mapEventData["binary"])
	_, hasDataKey := mapEventData["data"]
	require.False(t, hasDataKey, "map format should not have a 'data' key")

	arrayBody := formattedBody(event, EventDataFormatArray)
	arrayEventData := arrayBody["event_data"].(map[string]any)
	require.Equal(t, []any{
		map[string]any{"": "1st_value"},
		map[string]any{"": "2nd_value"},
	}, arrayEventData["data"])
	require.Equal(t, "2D20", arrayEventData["binary"])
}

func TestUnmarshalAndFormatNamedEventData(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xmlSample.xml"))
	require.NoError(t, err)

	event, err := unmarshalEventXML(data)
	require.NoError(t, err)

	mapBody := formattedBody(event, EventDataFormatMap)
	mapEventData := mapBody["event_data"].(map[string]any)
	require.Equal(t, "2022-04-28T19:48:52Z", mapEventData["Time"])
	require.Equal(t, "RulesEngine", mapEventData["Source"])

	arrayBody := formattedBody(event, EventDataFormatArray)
	arrayEventData := arrayBody["event_data"].(map[string]any)
	require.Equal(t, []any{
		map[string]any{"Time": "2022-04-28T19:48:52Z"},
		map[string]any{"Source": "RulesEngine"},
	}, arrayEventData["data"])
}

func TestParseEventDataSingleAnonymous(t *testing.T) {
	input := EventData{
		Data: []Data{{Value: "Test log"}},
	}
	result := parseEventData(input, EventDataFormatMap)
	require.Equal(t, map[string]any{"param1": "Test log"}, result)
}
