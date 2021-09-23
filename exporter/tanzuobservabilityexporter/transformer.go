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

package tanzuobservabilityexporter

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wavefronthq/wavefront-sdk-go/senders"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
)

type traceTransformer struct {
	resAttrs pdata.AttributeMap
}

func newTraceTransformer(resource pdata.Resource) *traceTransformer {
	t := &traceTransformer{
		resAttrs: resource.Attributes(),
	}
	return t
}

var (
	errInvalidSpanID  = errors.New("SpanID is invalid")
	errInvalidTraceID = errors.New("TraceID is invalid")
)

type span struct {
	Name           string
	TraceID        uuid.UUID
	SpanID         uuid.UUID
	ParentSpanID   uuid.UUID
	Tags           map[string]string
	StartMillis    int64
	DurationMillis int64
	SpanLogs       []senders.SpanLog
}

func (t *traceTransformer) Span(orig pdata.Span) (span, error) {
	traceID, err := traceIDtoUUID(orig.TraceID())
	if err != nil {
		return span{}, errInvalidTraceID
	}

	spanID, err := spanIDtoUUID(orig.SpanID())
	if err != nil {
		return span{}, errInvalidSpanID
	}

	startMillis, durationMillis := calculateTimes(orig)

	tags := attributesToTags(t.resAttrs, orig.Attributes())
	t.setRequiredTags(tags)

	tags[labelSpanKind] = spanKind(orig)

	errorTags := errorTagsFromStatus(orig.Status())
	for k, v := range errorTags {
		tags[k] = v
	}

	if len(orig.TraceState()) > 0 {
		tags[tracetranslator.TagW3CTraceState] = string(orig.TraceState())
	}

	return span{
		Name:           orig.Name(),
		TraceID:        traceID,
		SpanID:         spanID,
		ParentSpanID:   parentSpanIDtoUUID(orig.ParentSpanID()),
		Tags:           tags,
		StartMillis:    startMillis,
		DurationMillis: durationMillis,
		SpanLogs:       eventsToLogs(orig.Events()),
	}, nil
}

func spanKind(span pdata.Span) string {
	switch span.Kind() {
	case pdata.SpanKindClient:
		return "client"
	case pdata.SpanKindServer:
		return "server"
	case pdata.SpanKindProducer:
		return "producer"
	case pdata.SpanKindConsumer:
		return "consumer"
	case pdata.SpanKindInternal:
		return "internal"
	case pdata.SpanKindUnspecified:
		return "unspecified"
	default:
		return "unknown"
	}
}

func (t *traceTransformer) setRequiredTags(tags map[string]string) {
	if _, ok := tags[labelService]; !ok {
		if svcName, svcNameOk := tags[conventions.AttributeServiceName]; svcNameOk {
			tags[labelService] = svcName
			delete(tags, conventions.AttributeServiceName)
		} else {
			tags[labelService] = defaultServiceName
		}
	}
	if _, ok := tags[labelApplication]; !ok {
		tags[labelApplication] = defaultApplicationName
	}
}

func eventsToLogs(events pdata.SpanEventSlice) []senders.SpanLog {
	var result []senders.SpanLog
	for i := 0; i < events.Len(); i++ {
		e := events.At(i)
		fields := attributesToTags(e.Attributes())
		fields[labelEventName] = e.Name()
		result = append(result, senders.SpanLog{
			Timestamp: int64(e.Timestamp()) / time.Microsecond.Nanoseconds(), // Timestamp is in microseconds
			Fields:    fields,
		})
	}

	return result
}

func calculateTimes(span pdata.Span) (int64, int64) {
	startMillis := int64(span.StartTimestamp()) / time.Millisecond.Nanoseconds()
	endMillis := int64(span.EndTimestamp()) / time.Millisecond.Nanoseconds()
	durationMillis := endMillis - startMillis
	// it's possible end time is unset, so default to 0 rather than using a negative number
	if span.EndTimestamp() == 0 {
		durationMillis = 0
	}
	return startMillis, durationMillis
}

func attributesToTags(attributes ...pdata.AttributeMap) map[string]string {
	tags := map[string]string{}

	extractTag := func(k string, v pdata.AttributeValue) bool {
		tags[k] = v.AsString()
		return true
	}

	// Since AttributeMaps are processed in the order received, later values overwrite earlier ones
	for _, att := range attributes {
		att.Range(extractTag)
	}

	return tags
}

func errorTagsFromStatus(status pdata.SpanStatus) map[string]string {
	tags := map[string]string{
		labelStatusCode: fmt.Sprintf("%d", status.Code()),
	}
	if status.Code() != pdata.StatusCodeError {
		return tags
	}

	tags[labelError] = "true"
	if status.Message() != "" {
		msg := status.Message()
		const maxLength = 255 - len(labelStatusMessage+"=")
		if len(msg) > maxLength {
			msg = msg[:maxLength]
		}
		tags[labelStatusMessage] = msg
	}
	return tags
}

func traceIDtoUUID(id pdata.TraceID) (uuid.UUID, error) {
	formatted, err := uuid.Parse(id.HexString())
	if err != nil || id.IsEmpty() {
		return uuid.Nil, errInvalidTraceID
	}
	return formatted, nil
}

func spanIDtoUUID(id pdata.SpanID) (uuid.UUID, error) {
	formatted, err := uuid.FromBytes(padTo16Bytes(id.Bytes()))
	if err != nil || id.IsEmpty() {
		return uuid.Nil, errInvalidSpanID
	}
	return formatted, nil
}

func parentSpanIDtoUUID(id pdata.SpanID) uuid.UUID {
	if id.IsEmpty() {
		return uuid.Nil
	}
	// FromBytes only returns an error if the length is not 16 bytes, so the error case is unreachable
	formatted, _ := uuid.FromBytes(padTo16Bytes(id.Bytes()))
	return formatted
}

func padTo16Bytes(b [8]byte) []byte {
	as16bytes := make([]byte, 16)
	copy(as16bytes[16-len(b):], b[:])
	return as16bytes
}
