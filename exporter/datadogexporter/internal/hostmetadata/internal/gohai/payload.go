// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gohai // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/internal/gohai"

import (
	"encoding/json"
)

type gohai struct {
	CPU        interface{} `json:"cpu"`
	FileSystem interface{} `json:"filesystem"`
	Memory     interface{} `json:"memory"`
	Network    interface{} `json:"network"`
	Platform   interface{} `json:"platform"`
}

// Payload handles the JSON unmarshalling of the metadata payload
// As weird as it sounds, in the v5 payload the value of the "gohai" field
// is a JSON-formatted string. So this struct contains a MarshaledGohaiPayload
// which will be marshaled as a JSON-formatted string.
type Payload struct {
	Gohai gohaiMarshaler `json:"gohai"`
}

// gohaiSerializer implements json.Marshaler and json.Unmarshaler on top of a gohai payload
type gohaiMarshaler struct {
	gohai *gohai
}

// MarshalJSON implements the json.Marshaler interface.
// It marshals the gohai struct twice (to a string) to comply with
// the v5 payload format
func (m gohaiMarshaler) MarshalJSON() ([]byte, error) {
	marshaledPayload, err := json.Marshal(m.gohai)
	if err != nil {
		return []byte(""), err
	}
	doubleMarshaledPayload, err := json.Marshal(string(marshaledPayload))
	if err != nil {
		return []byte(""), err
	}
	return doubleMarshaledPayload, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Unmarshals the passed bytes twice (first to a string, then to gohai.Gohai)
func (m *gohaiMarshaler) UnmarshalJSON(bytes []byte) error {
	firstUnmarshall := ""
	err := json.Unmarshal(bytes, &firstUnmarshall)
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(firstUnmarshall), &(m.gohai))
	return err
}
