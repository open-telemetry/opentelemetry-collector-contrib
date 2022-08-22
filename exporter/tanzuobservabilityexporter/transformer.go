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

package tanzuobservabilityexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter"

import (
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/wavefronthq/wavefront-sdk-go/senders"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
)

type traceTransformer struct {
	resAttrs pcommon.Map
}

func newTraceTransformer(resource pcommon.Resource) *traceTransformer {
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
	Source         string
}

func (t *traceTransformer) Span(orig ptrace.Span) (span, error) {
	traceID, err := traceIDtoUUID(orig.TraceID())
	if err != nil {
		return span{}, errInvalidTraceID
	}

	spanID, err := spanIDtoUUID(orig.SpanID())
	if err != nil {
		return span{}, errInvalidSpanID
	}

	startMillis, durationMillis := calculateTimes(orig)

	source, attributesWithoutSource := getSourceAndResourceTags(t.resAttrs)
	tags := attributesToTagsReplaceSource(
		newMap(attributesWithoutSource), orig.Attributes())
	t.setRequiredTags(tags)

	tags[labelSpanKind] = spanKind(orig)

	if droppedEventsCount := orig.DroppedEventsCount(); droppedEventsCount > 0 {
		tags[labelDroppedEventsCount] = strconv.FormatUint(uint64(droppedEventsCount), 10)
	}

	if droppedLinksCount := orig.DroppedLinksCount(); droppedLinksCount > 0 {
		tags[labelDroppedLinksCount] = strconv.FormatUint(uint64(droppedLinksCount), 10)
	}

	if droppedAttrsCount := orig.DroppedAttributesCount(); droppedAttrsCount > 0 {
		tags[labelDroppedAttrsCount] = strconv.FormatUint(uint64(droppedAttrsCount), 10)
	}

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
		Source:         source,
		StartMillis:    startMillis,
		DurationMillis: durationMillis,
		SpanLogs:       eventsToLogs(orig.Events()),
	}, nil
}

func getSourceAndResourceTagsAndSourceKey(attributes pcommon.Map) (
	string, map[string]string, string) {
	attributesWithoutSource := map[string]string{}
	attributes.Range(func(k string, v pcommon.Value) bool {
		attributesWithoutSource[k] = v.AsString()
		return true
	})
	candidateKeys := []string{labelSource, conventions.AttributeHostName, "hostname", conventions.AttributeHostID}
	var source string
	var sourceKey string
	for _, key := range candidateKeys {
		if value, isFound := attributesWithoutSource[key]; isFound {
			source = value
			sourceKey = key
			delete(attributesWithoutSource, key)
			break
		}
	}

	// returning an empty source is fine as wavefront.go.sdk will set it up to a default value(os.hostname())
	return source, attributesWithoutSource, sourceKey
}

func getSourceAndResourceTags(attributes pcommon.Map) (string, map[string]string) {
	source, attributesWithoutSource, _ := getSourceAndResourceTagsAndSourceKey(attributes)
	return source, attributesWithoutSource
}

func getSourceAndKey(attributes pcommon.Map) (string, string) {
	source, _, sourceKey := getSourceAndResourceTagsAndSourceKey(attributes)
	return source, sourceKey
}

func spanKind(span ptrace.Span) string {
	switch span.Kind() {
	case ptrace.SpanKindClient:
		return "client"
	case ptrace.SpanKindServer:
		return "server"
	case ptrace.SpanKindProducer:
		return "producer"
	case ptrace.SpanKindConsumer:
		return "consumer"
	case ptrace.SpanKindInternal:
		return "internal"
	case ptrace.SpanKindUnspecified:
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

func eventsToLogs(events ptrace.SpanEventSlice) []senders.SpanLog {
	var result []senders.SpanLog
	for i := 0; i < events.Len(); i++ {
		e := events.At(i)
		fields := attributesToTagsReplaceSource(e.Attributes())
		fields[labelEventName] = e.Name()
		result = append(result, senders.SpanLog{
			Timestamp: int64(e.Timestamp()) / time.Microsecond.Nanoseconds(), // Timestamp is in microseconds
			Fields:    fields,
		})
	}

	return result
}

func calculateTimes(span ptrace.Span) (int64, int64) {
	startMillis := int64(span.StartTimestamp()) / time.Millisecond.Nanoseconds()
	endMillis := int64(span.EndTimestamp()) / time.Millisecond.Nanoseconds()
	durationMillis := endMillis - startMillis
	// it's possible end time is unset, so default to 0 rather than using a negative number
	if span.EndTimestamp() == 0 {
		durationMillis = 0
	}
	return startMillis, durationMillis
}

func attributesToTags(attributes ...pcommon.Map) map[string]string {
	tags := map[string]string{}
	for _, att := range attributes {
		att.Range(func(k string, v pcommon.Value) bool {
			tags[k] = v.AsString()
			return true
		})
	}
	return tags
}

func replaceSource(tags map[string]string) {
	if value, isFound := tags[labelSource]; isFound {
		delete(tags, labelSource)
		tags["_source"] = value
	}
}

func attributesToTagsReplaceSource(attributes ...pcommon.Map) map[string]string {
	tags := attributesToTags(attributes...)
	replaceSource(tags)
	return tags
}

func newMap(tags map[string]string) pcommon.Map {
	valueMap := make(map[string]interface{}, len(tags))
	for key, value := range tags {
		valueMap[key] = value
	}
	return pcommon.NewMapFromRaw(valueMap)
}

func errorTagsFromStatus(status ptrace.SpanStatus) map[string]string {
	tags := make(map[string]string)

	if status.Code() != ptrace.StatusCodeError {
		return tags
	}

	tags[labelError] = "true"

	if status.Message() != "" {
		msg := status.Message()
		const maxLength = 255 - len(conventions.OtelStatusDescription+"=")
		if len(msg) > maxLength {
			msg = msg[:maxLength]
		}
		tags[conventions.OtelStatusDescription] = msg
	}
	return tags
}

func traceIDtoUUID(id pcommon.TraceID) (uuid.UUID, error) {
	formatted, err := uuid.Parse(id.HexString())
	if err != nil || id.IsEmpty() {
		return uuid.Nil, errInvalidTraceID
	}
	return formatted, nil
}

func spanIDtoUUID(id pcommon.SpanID) (uuid.UUID, error) {
	formatted, err := uuid.FromBytes(padTo16Bytes(id.Bytes()))
	if err != nil || id.IsEmpty() {
		return uuid.Nil, errInvalidSpanID
	}
	return formatted, nil
}

func parentSpanIDtoUUID(id pcommon.SpanID) uuid.UUID {
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
