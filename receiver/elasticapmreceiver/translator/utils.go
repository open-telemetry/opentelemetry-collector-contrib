package translator

import (
	"encoding/hex"

	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func PutOptionalBool(attrs pcommon.Map, key string, value *bool) {
	if value != nil {
		attrs.PutBool(key, *value)
	}
}

func PutOptionalStr(attrs pcommon.Map, key string, value *string) {
	if value != nil && *value != "" {
		attrs.PutStr(key, *value)
	}
}

type Integer interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64
}

func PutOptionalInt[T Integer](attrs pcommon.Map, key string, value *T) {
	if value != nil {
		attrs.PutInt(key, int64(*value))
	}
}

func PutOptionalFloat[T float32 | float64](attrs pcommon.Map, key string, value *T) {
	if value != nil {
		attrs.PutDouble(key, float64(*value))
	}
}

func PutStr(attrs pcommon.Map, key string, value string) {
	if value == "" {
		attrs.PutStr(key, value)
	}
}

func PutStrArray(attrs pcommon.Map, key string, value []string) {
	if value == nil {
		return
	}
	for _, v := range value {
		attrs.PutStr(key, v)
	}
}

func PutMap(attrs pcommon.Map, m map[string]interface{}) {
	for k, v := range m {
		switch v.(type) {
		case string:
			attrs.PutStr(k, v.(string))
		case bool:
			attrs.PutBool(k, v.(bool))
		case float64:
			attrs.PutDouble(k, v.(float64))
		case int64:
			attrs.PutInt(k, v.(int64))
		default:
			attrs.PutStr(k, v.(string))
		}
	}
}

func PutLabelValue(attrs pcommon.Map, m map[string]*modelpb.LabelValue) {
	for k, v := range m {
		val := v.GetValue()
		if val != "" {
			attrs.PutStr(k, val)
		}
		values := v.GetValues()
		if values != nil {
			for _, value := range values {
				attrs.PutStr(k, value)
			}
		}
	}
}

func PutNumericLabelValue(attrs pcommon.Map, m map[string]*modelpb.NumericLabelValue) {
	for k, v := range m {
		val := v.GetValue()
		if val != 0 {
			attrs.PutDouble(k, val)
		}
		values := v.GetValues()
		if values != nil {
			for _, value := range values {
				attrs.PutDouble(k, value)
			}
		}
	}
}

func ConvertTraceId(src string) pcommon.TraceID {
	var dest pcommon.TraceID
	traceId, _ := hex.DecodeString(src)
	copy(dest[:], traceId)
	return dest
}

func ConvertSpanId(src string) pcommon.SpanID {
	var dest pcommon.SpanID
	spanId, _ := hex.DecodeString(src)
	copy(dest[:], spanId)
	return dest
}

func ConvertSpanKind(src string) ptrace.SpanKind {
	switch src {
	case "client":
		return ptrace.SpanKindClient
	case "server":
		return ptrace.SpanKindServer
	case "producer":
		return ptrace.SpanKindProducer
	case "consumer":
		return ptrace.SpanKindConsumer
	case "internal":
		return ptrace.SpanKindInternal
	default:
		return ptrace.SpanKindUnspecified
	}
}

func GetStartAndEndTimestamps(timestamp *timestamppb.Timestamp, duration *durationpb.Duration) (*pcommon.Timestamp, *pcommon.Timestamp) {
	if timestamp != nil {
		startTime := timestamp.AsTime()
		endTime := startTime.Add(duration.AsDuration())
		// If timestamp is set, use it as the start time and calculate the end time
		start := pcommon.NewTimestampFromTime(startTime)
		end := pcommon.NewTimestampFromTime(endTime)
		return &start, &end
	}
	return nil, nil
}
