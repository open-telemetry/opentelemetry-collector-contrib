// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package codec provides encoding and decoding for the libhoney event format.
// It handles both JSON and MessagePack formats, supporting single events,
// batches, flat events, and structured events with header-based metadata.
package codec // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/codec"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/eventtime"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/libhoneyevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/response"
)

const (
	JSONContentType    = "application/json"
	MsgpackContentType = "application/msgpack"
)

var (
	JsEncoder = &jsonEncoder{}
	MpEncoder = &msgpackEncoder{}
)

// Encoder handles marshaling of libhoney response batches in the appropriate format
type Encoder interface {
	MarshalResponse([]response.ResponseInBatch) ([]byte, error)
	ContentType() string
}

type jsonEncoder struct{}

func (jsonEncoder) MarshalResponse(batchResponse []response.ResponseInBatch) ([]byte, error) {
	return json.Marshal(batchResponse)
}

func (jsonEncoder) ContentType() string {
	return JSONContentType
}

type msgpackEncoder struct{}

func (msgpackEncoder) MarshalResponse(batchResponse []response.ResponseInBatch) ([]byte, error) {
	return msgpack.Marshal(batchResponse)
}

func (msgpackEncoder) ContentType() string {
	return MsgpackContentType
}

// GetEncoder returns the appropriate encoder for the given content type
func GetEncoder(contentType string) (Encoder, error) {
	switch contentType {
	case JSONContentType:
		return JsEncoder, nil
	case MsgpackContentType, "application/x-msgpack":
		return MpEncoder, nil
	default:
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
}

// DecodeEvents decodes libhoney events from the request body based on content type
func DecodeEvents(contentType string, body []byte, headers http.Header) ([]libhoneyevent.LibhoneyEvent, error) {
	switch contentType {
	case "application/x-msgpack", "application/msgpack":
		return decodeMsgpack(body, headers)
	case JSONContentType:
		return decodeJSON(body, headers)
	default:
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
}

// decodeMsgpack decodes msgpack-encoded libhoney events
func decodeMsgpack(body []byte, headers http.Header) ([]libhoneyevent.LibhoneyEvent, error) {
	if isSingleMsgpackObject(body) {
		return decodeSingleMsgpackEvent(body, headers)
	}
	return decodeMsgpackArray(body)
}

// decodeJSON decodes JSON-encoded libhoney events
func decodeJSON(body []byte, headers http.Header) ([]libhoneyevent.LibhoneyEvent, error) {
	// Check first non-whitespace character to determine structure
	trimmed := bytes.TrimLeft(body, " \t\n\r")
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return decodeSingleJSONEvent(body, headers)
	}
	return decodeJSONArray(body)
}

// isSingleMsgpackObject checks if the msgpack data represents a single object (map) vs an array
// msgpack format: 0x80-0x8f = fixmap, 0xde = map16, 0xdf = map32
//
//	0x90-0x9f = fixarray, 0xdc = array16, 0xdd = array32
func isSingleMsgpackObject(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	firstByte := body[0]
	// Check if it's a map type
	return (firstByte >= 0x80 && firstByte <= 0x8f) || firstByte == 0xde || firstByte == 0xdf
}

// decodeSingleMsgpackEvent decodes a single msgpack event
func decodeSingleMsgpackEvent(body []byte, headers http.Header) ([]libhoneyevent.LibhoneyEvent, error) {
	var rawEvent map[string]any
	decoder := msgpack.NewDecoder(bytes.NewReader(body))
	decoder.UseLooseInterfaceDecoding(true)
	err := decoder.Decode(&rawEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to decode msgpack: %w", err)
	}

	// Apply headers for single event (X-Honeycomb-Event-Time, X-Honeycomb-Samplerate)
	applyHeadersToEvent(rawEvent, headers)

	// Wrap flat event if needed
	rawEvent = wrapFlatEventIfNeeded(rawEvent)

	// Construct LibhoneyEvent directly from the map
	event := buildLibhoneyEventFromMap(rawEvent)
	return []libhoneyevent.LibhoneyEvent{event}, nil
}

// decodeMsgpackArray decodes an array of msgpack events
func decodeMsgpackArray(body []byte) ([]libhoneyevent.LibhoneyEvent, error) {
	var rawEvents []map[string]any
	decoder := msgpack.NewDecoder(bytes.NewReader(body))
	decoder.UseLooseInterfaceDecoding(true)
	err := decoder.Decode(&rawEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to decode msgpack array: %w (hex: %x)", err, body[:min(len(body), 100)])
	}

	events := make([]libhoneyevent.LibhoneyEvent, 0, len(rawEvents))
	for _, rawEvent := range rawEvents {
		event := buildLibhoneyEventFromMap(rawEvent)
		events = append(events, event)
	}
	return events, nil
}

// decodeSingleJSONEvent decodes a single JSON event
func decodeSingleJSONEvent(body []byte, headers http.Header) ([]libhoneyevent.LibhoneyEvent, error) {
	var rawEvent map[string]any
	err := json.Unmarshal(body, &rawEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Apply headers for single event (X-Honeycomb-Event-Time, X-Honeycomb-Samplerate)
	applyHeadersToEvent(rawEvent, headers)

	// Wrap flat event if needed
	rawEvent = wrapFlatEventIfNeeded(rawEvent)

	// Now unmarshal the structured event
	rawBytes, _ := json.Marshal(rawEvent)
	var singleEvent libhoneyevent.LibhoneyEvent
	err = json.Unmarshal(rawBytes, &singleEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal structured event: %w", err)
	}

	return []libhoneyevent.LibhoneyEvent{singleEvent}, nil
}

// decodeJSONArray decodes an array of JSON events
func decodeJSONArray(body []byte) ([]libhoneyevent.LibhoneyEvent, error) {
	var events []libhoneyevent.LibhoneyEvent
	err := json.Unmarshal(body, &events)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON array: %w", err)
	}
	return events, nil
}

// applyHeadersToEvent applies X-Honeycomb-Event-Time and X-Honeycomb-Samplerate headers to the event
// Only applies if not already present in the body
func applyHeadersToEvent(rawEvent map[string]any, headers http.Header) {
	if eventTimeHeader := headers.Get("X-Honeycomb-Event-Time"); eventTimeHeader != "" {
		if _, hasTime := rawEvent["time"]; !hasTime {
			rawEvent["time"] = eventTimeHeader
		}
	}

	if samplerateHeader := headers.Get("X-Honeycomb-Samplerate"); samplerateHeader != "" {
		if _, hasSamplerate := rawEvent["samplerate"]; !hasSamplerate {
			// Convert string to number for compatibility
			var samplerate any
			if sr, convErr := json.Number(samplerateHeader).Int64(); convErr == nil {
				samplerate = sr
			} else if sr, convErr := json.Number(samplerateHeader).Float64(); convErr == nil {
				samplerate = sr
			} else {
				samplerate = samplerateHeader // Fallback to string
			}
			rawEvent["samplerate"] = samplerate
		}
	}
}

// wrapFlatEventIfNeeded wraps a flat event (one without a "data" field) into the structured format
func wrapFlatEventIfNeeded(rawEvent map[string]any) map[string]any {
	if _, hasData := rawEvent["data"]; hasData {
		// Already has a data field, no wrapping needed
		return rawEvent
	}

	// Flat event format - wrap all fields into data
	dataCopy := make(map[string]any)
	for k, v := range rawEvent {
		if k != "time" && k != "samplerate" {
			dataCopy[k] = v
		}
	}

	wrappedEvent := make(map[string]any)
	wrappedEvent["data"] = dataCopy

	// Preserve time and samplerate at top level if they exist
	if t, ok := rawEvent["time"]; ok {
		wrappedEvent["time"] = t
	}
	if sr, ok := rawEvent["samplerate"]; ok {
		wrappedEvent["samplerate"] = sr
	}

	return wrappedEvent
}

// buildLibhoneyEventFromMap constructs a LibhoneyEvent from a raw map
func buildLibhoneyEventFromMap(rawEvent map[string]any) libhoneyevent.LibhoneyEvent {
	event := libhoneyevent.LibhoneyEvent{
		Samplerate: 1, // default
		Data:       make(map[string]any),
	}

	// Extract samplerate
	if sr, ok := rawEvent["samplerate"]; ok {
		switch v := sr.(type) {
		case int:
			event.Samplerate = v
		case int64:
			event.Samplerate = int(v)
		case uint64:
			event.Samplerate = int(v)
		case float64:
			event.Samplerate = int(v)
		}
	}

	// Extract time
	if t, ok := rawEvent["time"]; ok {
		switch v := t.(type) {
		case string:
			event.Time = v
			propertime := eventtime.GetEventTime(v)
			event.MsgPackTimestamp = &propertime
		case *time.Time:
			event.MsgPackTimestamp = v
		case time.Time:
			event.MsgPackTimestamp = &v
		}
	}
	if event.MsgPackTimestamp == nil {
		tnow := time.Now()
		event.MsgPackTimestamp = &tnow
	}

	// Extract data
	if data, ok := rawEvent["data"].(map[string]any); ok {
		event.Data = data
	}

	return event
}

// ValidateEvents performs basic validation on decoded events
func ValidateEvents(events []libhoneyevent.LibhoneyEvent) error {
	if len(events) == 0 {
		return errors.New("no events decoded")
	}
	return nil
}
