// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	marshaler := ptrace.ProtoMarshaler{}
	data, _ := marshaler.MarshalTraces(out)
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
	marshaler := pmetric.ProtoMarshaler{}
	data, _ := marshaler.MarshalMetrics(out)
	return data
}

func CreateLogExport() []byte {
	out := plog.NewLogs()
	resources := out.ResourceLogs()
	resource := resources.AppendEmpty()
	libs := resource.ScopeLogs()
	logs := libs.AppendEmpty()
	logs.LogRecords().AppendEmpty()
	marshaler := plog.ProtoMarshaler{}
	data, _ := marshaler.MarshalLogs(out)
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
