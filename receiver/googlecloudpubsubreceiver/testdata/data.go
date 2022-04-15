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

package testdata

import (
	"bytes"
	"compress/gzip"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func CreateTraceExport() []byte {
	out := ptrace.NewTraces()
	resources := out.ResourceSpans()
	resource := resources.AppendEmpty()
	libs := resource.ScopeSpans()
	spans := libs.AppendEmpty().Spans()
	span := spans.AppendEmpty()
	span.SetName("test")
	data, _ := ptrace.NewProtoMarshaler().MarshalTraces(out)
	return data
}

func CreateMetricExport() []byte {
	out := pmetric.NewMetrics()
	resources := out.ResourceMetrics()
	resource := resources.AppendEmpty()
	libs := resource.ScopeMetrics()
	metrics := libs.AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName("test")
	data, _ := pmetric.NewProtoMarshaler().MarshalMetrics(out)
	return data
}

func CreateLogExport() []byte {
	out := plog.NewLogs()
	resources := out.ResourceLogs()
	resource := resources.AppendEmpty()
	libs := resource.ScopeLogs()
	logs := libs.AppendEmpty()
	log := logs.LogRecords().AppendEmpty()
	log.SetName("test")
	data, _ := plog.NewProtoMarshaler().MarshalLogs(out)
	return data
}

func CreateGZippedLogExport() []byte {
	payload := CreateLogExport()
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	writer.Write(payload)
	return buf.Bytes()
}

func CreateTextExport() []byte {
	return []byte("this is text")
}
