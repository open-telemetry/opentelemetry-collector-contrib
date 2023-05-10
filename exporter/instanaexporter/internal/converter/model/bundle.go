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
