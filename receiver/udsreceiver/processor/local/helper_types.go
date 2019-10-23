// Copyright 2019, OpenTelemetry Authors
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

package local

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udsreceiver"
)

type measurement struct {
	name  string
	mType udsreceiver.MeasurementType
	val   interface{}
}

type span struct {
	TraceID      string      `json:"traceId"`
	SpanID       string      `json:"spanId"`
	ParentSpanID string      `json:"parentSpanId"`
	Name         string      `json:"name"`
	Kind         string      `json:"kind"`
	StackTrace   []string    `json:"stackTrace"`
	StartTime    dateTime    `json:"startTime"`
	EndTime      dateTime    `json:"endTime"`
	Status       status      `json:"status"`
	Attributes   attributes  `json:"attributes"`
	TimeEvents   interface{} `json:"timeEvents"`
	Links        []link      `json:"links"`
	SameProcess  bool        `json:"sameProcessAsParentSpan"`
}

type dateTime time.Time

func (dt *dateTime) UnmarshalJSON(data []byte) error {
	if dt == nil {
		return nil
	}
	var pdt struct {
		Date string `json:"date"`
	}
	if err := json.Unmarshal(data, &pdt); err != nil {
		return err
	}
	gdt, err := time.Parse("2006-01-02 15:04:05.000000", pdt.Date)
	if err != nil {
		return err
	}
	*dt = dateTime(gdt)
	return nil
}

type status struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
}

type attributes map[string]interface{}

func (a *attributes) UnmarshalJSON(data []byte) error {
	type alias map[string]interface{}
	if a == nil {
		return nil
	}
	if bytes.Equal(data, []byte("[]")) || bytes.Equal(data, []byte("null")) {
		return nil
	}
	var val alias
	err := json.Unmarshal(data, &val)
	if err != nil {
		return err
	}
	*a = map[string]interface{}(val)
	return nil
}

type link struct {
	TraceID    []byte            `json:"traceId"`
	SpanID     []byte            `json:"spanId"`
	Type       string            `json:"type"`
	Attributes map[string]string `json:"attributes"`
}
