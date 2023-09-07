// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const (
	OtelSpanType = "otel"

	InstanaSpanKindServer   = "server"
	InstanaSpanKindClient   = "client"
	InstanaSpanKindProducer = "producer"
	InstanaSpanKindConsumer = "consumer"
	InstanaSpanKindInternal = "internal"

	InstanaDataError       = "error"
	InstanaDataErrorDetail = "error_detail"
)

type BatchInfo struct {
	Size int `json:"s"`
}

type FromS struct {
	EntityID string `json:"e"`
	// Serverless agents fields
	Hostless      bool   `json:"hl,omitempty"`
	CloudProvider string `json:"cp,omitempty"`
	// Host agent fields
	HostID string `json:"h,omitempty"`
}

type TraceReference struct {
	TraceID  string `json:"t"`
	ParentID string `json:"p,omitempty"`
}

type OTelSpanData struct {
	Kind           string            `json:"kind"`
	HasTraceParent bool              `json:"tp,omitempty"`
	ServiceName    string            `json:"service"`
	Operation      string            `json:"operation"`
	TraceState     string            `json:"trace_state,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
	Resource       map[string]string `json:"resource,omitempty"`
}

type Span struct {
	TraceReference

	SpanID          string          `json:"s"`
	LongTraceID     string          `json:"lt,omitempty"`
	Timestamp       uint64          `json:"ts"`
	Duration        uint64          `json:"d"`
	Name            string          `json:"n"`
	From            *FromS          `json:"f"`
	Batch           *BatchInfo      `json:"b,omitempty"`
	Ec              int             `json:"ec,omitempty"`
	Synthetic       bool            `json:"sy,omitempty"`
	CorrelationType string          `json:"crtp,omitempty"`
	CorrelationID   string          `json:"crid,omitempty"`
	ForeignTrace    bool            `json:"tp,omitempty"`
	Ancestor        *TraceReference `json:"ia,omitempty"`
	Data            OTelSpanData    `json:"data,omitempty"`
}

func ConvertPDataSpanToInstanaSpan(fromS FromS, otelSpan ptrace.Span, serviceName string, attributes pcommon.Map) (Span, error) {
	traceID := convertTraceID(otelSpan.TraceID())

	instanaSpan := Span{
		Name:           OtelSpanType,
		TraceReference: TraceReference{},
		Timestamp:      uint64(otelSpan.StartTimestamp()) / uint64(time.Millisecond),
		Duration:       (uint64(otelSpan.EndTimestamp()) - uint64(otelSpan.StartTimestamp())) / uint64(time.Millisecond),
		Data: OTelSpanData{
			Tags:     make(map[string]string),
			Resource: make(map[string]string),
		},
		From: &fromS,
	}

	if len(traceID) != 32 {
		return Span{}, fmt.Errorf("failed parsing span, length of TraceID should be 32, but got %d", len(traceID))
	}

	instanaSpan.TraceReference.TraceID = traceID[16:32]
	instanaSpan.LongTraceID = traceID

	if !otelSpan.ParentSpanID().IsEmpty() {
		instanaSpan.TraceReference.ParentID = convertSpanID(otelSpan.ParentSpanID())
	}

	instanaSpan.SpanID = convertSpanID(otelSpan.SpanID())

	kind, isEntry := otelKindToInstanaKind(otelSpan.Kind())
	instanaSpan.Data.Kind = kind

	if !otelSpan.ParentSpanID().IsEmpty() && isEntry {
		instanaSpan.Data.HasTraceParent = true
	}

	instanaSpan.Data.ServiceName = serviceName

	instanaSpan.Data.Operation = otelSpan.Name()

	instanaSpan.Data.TraceState = otelSpan.TraceState().AsRaw()

	otelSpan.Attributes().Range(func(k string, v pcommon.Value) bool {
		instanaSpan.Data.Tags[k] = v.AsString()

		return true
	})

	attributes.Range(func(k string, v pcommon.Value) bool {
		instanaSpan.Data.Resource[k] = v.AsString()

		return true
	})

	errornous := false
	if otelSpan.Status().Code() == ptrace.StatusCodeError {
		errornous = true
		instanaSpan.Data.Tags[InstanaDataError] = traceutil.StatusCodeStr(otelSpan.Status().Code())
		instanaSpan.Data.Tags[InstanaDataErrorDetail] = otelSpan.Status().Message()
	}

	if errornous {
		instanaSpan.Ec = 1
	}

	return instanaSpan, nil
}
