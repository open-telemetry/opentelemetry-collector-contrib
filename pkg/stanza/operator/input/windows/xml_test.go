// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestParseValidTimestamp(t *testing.T) {
	xml := EventXML{
		TimeCreated: TimeCreated{
			SystemTime: "2020-07-30T01:01:01.123456789Z",
		},
	}
	timestamp := xml.parseTimestamp()
	expected, _ := time.Parse(time.RFC3339Nano, "2020-07-30T01:01:01.123456789Z")
	require.Equal(t, expected, timestamp)
}

func TestParseInvalidTimestamp(t *testing.T) {
	xml := EventXML{
		TimeCreated: TimeCreated{
			SystemTime: "invalid",
		},
	}
	timestamp := xml.parseTimestamp()
	require.Equal(t, time.Now().Year(), timestamp.Year())
	require.Equal(t, time.Now().Month(), timestamp.Month())
	require.Equal(t, time.Now().Day(), timestamp.Day())
}

func TestParseSeverity(t *testing.T) {
	xmlRenderedCritical := EventXML{RenderedLevel: "Critical"}
	xmlRenderedError := EventXML{RenderedLevel: "Error"}
	xmlRenderedWarning := EventXML{RenderedLevel: "Warning"}
	xmlRenderedInformation := EventXML{RenderedLevel: "Information"}
	xmlRenderedUnknown := EventXML{RenderedLevel: "Unknown"}
	xmlCritical := EventXML{Level: "1"}
	xmlError := EventXML{Level: "2"}
	xmlWarning := EventXML{Level: "3"}
	xmlInformation := EventXML{Level: "4"}
	xmlUnknown := EventXML{Level: "0"}
	require.Equal(t, entry.Fatal, xmlRenderedCritical.parseRenderedSeverity())
	require.Equal(t, entry.Error, xmlRenderedError.parseRenderedSeverity())
	require.Equal(t, entry.Warn, xmlRenderedWarning.parseRenderedSeverity())
	require.Equal(t, entry.Info, xmlRenderedInformation.parseRenderedSeverity())
	require.Equal(t, entry.Default, xmlRenderedUnknown.parseRenderedSeverity())
	require.Equal(t, entry.Fatal, xmlCritical.parseRenderedSeverity())
	require.Equal(t, entry.Error, xmlError.parseRenderedSeverity())
	require.Equal(t, entry.Warn, xmlWarning.parseRenderedSeverity())
	require.Equal(t, entry.Info, xmlInformation.parseRenderedSeverity())
	require.Equal(t, entry.Default, xmlUnknown.parseRenderedSeverity())
}

func TestParseBody(t *testing.T) {
	xml := EventXML{
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
		Message:  "message",
		Task:     "task",
		Opcode:   "opcode",
		Keywords: []string{"keyword"},
		EventData: EventData{
			Data: []Data{{Name: "1st_name", Value: "value"}, {Name: "2nd_name", Value: "another_value"}},
		},
		RenderedLevel:    "rendered_level",
		RenderedTask:     "rendered_task",
		RenderedOpcode:   "rendered_opcode",
		RenderedKeywords: []string{"RenderedKeywords"},
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
			"data": []any{
				map[string]any{"1st_name": "value"},
				map[string]any{"2nd_name": "another_value"},
			},
		},
	}

	require.Equal(t, expected, xml.parseBody())
}

func TestParseBodySecurityExecution(t *testing.T) {
	xml := EventXML{
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
		Message:  "message",
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
		RenderedLevel:    "rendered_level",
		RenderedTask:     "rendered_task",
		RenderedOpcode:   "rendered_opcode",
		RenderedKeywords: []string{"RenderedKeywords"},
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
			"data": []any{
				map[string]any{"name": "value"},
				map[string]any{"another_name": "another_value"},
			},
		},
	}

	require.Equal(t, expected, xml.parseBody())
}

func TestParseBodyFullExecution(t *testing.T) {
	processorID := uint(3)
	sessionID := uint(2)
	kernelTime := uint(3)
	userTime := uint(100)
	processorTime := uint(200)

	xml := EventXML{
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
		Message:  "message",
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
		RenderedLevel:    "rendered_level",
		RenderedTask:     "rendered_task",
		RenderedOpcode:   "rendered_opcode",
		RenderedKeywords: []string{"RenderedKeywords"},
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
			"data": []any{
				map[string]any{"name": "value"},
				map[string]any{"another_name": "another_value"},
			},
		},
	}

	require.Equal(t, expected, xml.parseBody())
}

func TestParseNoRendered(t *testing.T) {
	xml := EventXML{
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
		Message:  "message",
		Task:     "task",
		Opcode:   "opcode",
		Keywords: []string{"keyword"},
		EventData: EventData{
			Data: []Data{{Name: "name", Value: "value"}, {Name: "another_name", Value: "another_value"}},
		},
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
		"message":     "message",
		"task":        "task",
		"opcode":      "opcode",
		"keywords":    []string{"keyword"},
		"event_data": map[string]any{
			"data": []any{
				map[string]any{"name": "value"},
				map[string]any{"another_name": "another_value"},
			},
		},
	}

	require.Equal(t, expected, xml.parseBody())
}

func TestParseBodySecurity(t *testing.T) {
	xml := EventXML{
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
		Message:  "message",
		Task:     "task",
		Opcode:   "opcode",
		Keywords: []string{"keyword"},
		EventData: EventData{
			Data: []Data{{Name: "name", Value: "value"}, {Name: "another_name", Value: "another_value"}},
		},
		RenderedLevel:    "rendered_level",
		RenderedTask:     "rendered_task",
		RenderedOpcode:   "rendered_opcode",
		RenderedKeywords: []string{"RenderedKeywords"},
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
			"data": []any{
				map[string]any{"name": "value"},
				map[string]any{"another_name": "another_value"},
			},
		},
	}

	require.Equal(t, expected, xml.parseBody())
}

func TestParseEventData(t *testing.T) {
	xmlMap := EventXML{
		EventData: EventData{
			Name:   "EVENT_DATA",
			Data:   []Data{{Name: "name", Value: "value"}},
			Binary: "2D20",
		},
	}

	parsed := xmlMap.parseBody()
	expectedMap := map[string]any{
		"name": "EVENT_DATA",
		"data": []any{
			map[string]any{"name": "value"},
		},
		"binary": "2D20",
	}
	require.Equal(t, expectedMap, parsed["event_data"])

	xmlMixed := EventXML{
		EventData: EventData{
			Data: []Data{{Name: "name", Value: "value"}, {Value: "no_name"}},
		},
	}

	parsed = xmlMixed.parseBody()
	expectedSlice := map[string]any{
		"data": []any{
			map[string]any{"name": "value"},
			map[string]any{"": "no_name"}},
	}
	require.Equal(t, expectedSlice, parsed["event_data"])
}

func TestInvalidUnmarshal(t *testing.T) {
	_, err := unmarshalEventXML([]byte("Test \n Invalid \t Unmarshal"))
	require.Error(t, err)

}
func TestUnmarshalWithEventData(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xmlSample.xml"))
	require.NoError(t, err)

	event, err := unmarshalEventXML(data)
	require.NoError(t, err)

	xml := EventXML{
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
		Message:   "",
		Task:      "0",
		Opcode:    "0",
		Execution: &Execution{},
		Security:  &Security{},
		EventData: EventData{
			Data: []Data{
				{Name: "Time", Value: "2022-04-28T19:48:52Z"},
				{Name: "Source", Value: "RulesEngine"}},
		},
		Keywords: []string{"0x80000000000000"},
	}

	require.Equal(t, xml, event)
}

func TestUnmarshalWithAnonymousEventDataEntries(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xmlWithAnonymousEventDataEntries.xml"))
	require.NoError(t, err)

	event, err := unmarshalEventXML(data)
	require.NoError(t, err)

	xml := EventXML{
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
		Message:  "",
		Task:     "0",
		Opcode:   "0",
		EventData: EventData{
			Data:   []Data{{Name: "", Value: "1st_value"}, {Name: "", Value: "2nd_value"}},
			Binary: "2D20",
		},
		Keywords:  []string{"0x80000000000000"},
		Security:  &Security{},
		Execution: &Execution{},
	}

	require.Equal(t, xml, event)
}

func TestUnmarshalWithUserData(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xmlSampleUserData.xml"))
	require.NoError(t, err)

	event, err := unmarshalEventXML(data)
	require.NoError(t, err)

	xml := EventXML{
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
		Message:  "",
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
	}

	require.Equal(t, xml, event)
}
