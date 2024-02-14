// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"

import (
	"encoding/json"
)

type Bundle struct {
	Spans []Span `json:"spans,omitempty"`
}

func NewBundle() Bundle {
	return Bundle{
		Spans: []Span{},
	}
}

func (b *Bundle) Marshal() ([]byte, error) {
	json, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}

	return json, nil
}
