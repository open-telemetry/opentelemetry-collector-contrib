// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadoglogreceiver/internal"

import (
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"
)

type UnixTime time.Time

// UnmarshalJSON is the method that satisfies the Unmarshaller interface
func (u *UnixTime) UnmarshalJSON(b []byte) error {
	var timestamp int64
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(b, &timestamp)
	if err != nil {
		return err
	}
	// DataDog timestamp is in milliseconds so we need to convert it to seconds
	*u = UnixTime(time.Unix(timestamp/1000, (timestamp%1000)*1_000_000))
	return nil
}

func (m *Message) UnmarshalJSON(b []byte) error {
	var v any
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	switch val := v.(type) {
	case string:
		m.Str = val
	case map[string]any:
		m.Map = val
	case []any:
		m.Arr = val
	default:
		return fmt.Errorf("unsupported message type: %T", v)
	}

	return nil
}

type Message struct {
	Str string
	Map map[string]any
	Arr []any
}

type DatadogRecord struct {
	Message   Message  `json:"message"`
	Status    string   `json:"status"`
	Timestamp UnixTime `json:"timestamp"`
	Hostname  string   `json:"hostname"`
	Service   string   `json:"service"`
	Ddsource  string   `json:"ddsource"`
	Ddtags    string   `json:"ddtags"`
}
