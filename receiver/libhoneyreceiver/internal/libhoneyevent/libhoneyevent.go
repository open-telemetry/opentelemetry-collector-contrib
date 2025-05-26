// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyevent // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/libhoneyevent"

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	trc "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/eventtime"
)

// FieldMapConfig is used to map the fields from the LibhoneyEvent to PData formats
type FieldMapConfig struct {
	Resources  ResourcesConfig  `mapstructure:"resources"`
	Scopes     ScopesConfig     `mapstructure:"scopes"`
	Attributes AttributesConfig `mapstructure:"attributes"`
}

// ResourcesConfig is used to map the fields from the LibhoneyEvent to PData formats
type ResourcesConfig struct {
	ServiceName string `mapstructure:"service_name"`
}

// ScopesConfig is used to map the fields from the LibhoneyEvent to PData formats
type ScopesConfig struct {
	LibraryName    string `mapstructure:"library_name"`
	LibraryVersion string `mapstructure:"library_version"`
}

// AttributesConfig is used to map the fields from the LibhoneyEvent to PData formats
type AttributesConfig struct {
	TraceID        string   `mapstructure:"trace_id"`
	ParentID       string   `mapstructure:"parent_id"`
	SpanID         string   `mapstructure:"span_id"`
	Name           string   `mapstructure:"name"`
	Error          string   `mapstructure:"error"`
	SpanKind       string   `mapstructure:"spankind"`
	DurationFields []string `mapstructure:"durationFields"`
}

// LibhoneyEvent is the event structure from libhoney
type LibhoneyEvent struct {
	Samplerate       int            `json:"samplerate" msgpack:"samplerate"`
	MsgPackTimestamp *time.Time     `msgpack:"time"`
	Time             string         `json:"time"` // should not be trusted. use MsgPackTimestamp
	Data             map[string]any `json:"data" msgpack:"data"`
}

// UnmarshalJSON overrides the unmarshall to make sure the MsgPackTimestamp is set
func (l *LibhoneyEvent) UnmarshalJSON(j []byte) error {
	type _libhoneyEvent LibhoneyEvent
	tstr := eventtime.GetEventTimeDefaultString()
	tzero := time.Time{}
	tmp := _libhoneyEvent{Time: "none", MsgPackTimestamp: &tzero, Samplerate: 1}

	err := json.Unmarshal(j, &tmp)
	if err != nil {
		return err
	}
	if tmp.MsgPackTimestamp.IsZero() && tmp.Time == "none" {
		// neither timestamp was set. give it right now.
		tmp.Time = tstr
		tnow := time.Now()
		tmp.MsgPackTimestamp = &tnow
	}
	if tmp.MsgPackTimestamp.IsZero() {
		propertime := eventtime.GetEventTime(tmp.Time)
		tmp.MsgPackTimestamp = &propertime
	}

	*l = LibhoneyEvent(tmp)
	return nil
}

// DebugString returns a string representation of the LibhoneyEvent
func (l *LibhoneyEvent) DebugString() string {
	return fmt.Sprintf("%#v", l)
}

// SignalType returns the type of signal this event represents. Only log is implemented for now.
func (l *LibhoneyEvent) SignalType(logger zap.Logger) string {
	if sig, ok := l.Data["meta.signal_type"]; ok {
		switch sig {
		case "trace":
			if atype, ok := l.Data["meta.annotation_type"]; ok {
				switch atype {
				case "span_event":
					return "span_event"
				case "link":
					return "span_link"
				}
				logger.Warn("invalid annotation type", zap.String("meta.annotation_type", atype.(string)))
				return "span"
			}
			return "span"
		case "log":
			return "log"
		default:
			logger.Warn("invalid meta.signal_type", zap.String("meta.signal_type", sig.(string)))
			return "log"
		}
	}
	logger.Warn("missing meta.signal_type and meta.annotation_type")
	return "log"
}

// GetService returns the service name from the event or the dataset name if no service name is found.
func (l *LibhoneyEvent) GetService(fields FieldMapConfig, seen *ServiceHistory, dataset string) (string, error) {
	if serviceName, ok := l.Data[fields.Resources.ServiceName]; ok {
		seen.NameCount[serviceName.(string)]++
		return serviceName.(string), nil
	}
	return dataset, errors.New("no service.name found in event")
}

// GetScope returns the scope key for the event. If the scope has not been seen before, it creates a new one.
func (l *LibhoneyEvent) GetScope(fields FieldMapConfig, seen *ScopeHistory, serviceName string) (string, error) {
	if scopeLibraryName, ok := l.Data[fields.Scopes.LibraryName]; ok {
		scopeKey := serviceName + scopeLibraryName.(string)
		if _, ok := seen.Scope[scopeKey]; ok {
			// if we've seen it, we don't expect it to be different right away so we'll just return it.
			return scopeKey, nil
		}
		// otherwise, we need to make a new found scope
		scopeLibraryVersion := "unset"
		if scopeLibVer, ok := l.Data[fields.Scopes.LibraryVersion]; ok {
			scopeLibraryVersion = scopeLibVer.(string)
		}
		newScope := SimpleScope{
			ServiceName:    serviceName, // we only set the service name once. If the same library comes from multiple services in the same batch, we're in trouble.
			LibraryName:    scopeLibraryName.(string),
			LibraryVersion: scopeLibraryVersion,
			ScopeSpans:     ptrace.NewSpanSlice(),
			ScopeLogs:      plog.NewLogRecordSlice(),
		}
		seen.Scope[scopeKey] = newScope
		return scopeKey, nil
	}
	return "libhoney.receiver", errors.New("library name not found")
}

func spanIDFrom(s string) trc.SpanID {
	hash := fnv.New64a()
	hash.Write([]byte(s))
	n := hash.Sum64()
	sid := trc.SpanID{}
	binary.LittleEndian.PutUint64(sid[:], n)
	return sid
}

func traceIDFrom(s string) trc.TraceID {
	hash := fnv.New64a()
	hash.Write([]byte(s))
	n1 := hash.Sum64()
	hash.Write([]byte(s))
	n2 := hash.Sum64()
	tid := trc.TraceID{}
	binary.LittleEndian.PutUint64(tid[:], n1)
	binary.LittleEndian.PutUint64(tid[8:], n2)
	return tid
}

func generateAnID(length int) []byte {
	token := make([]byte, length)
	_, err := rand.Read(token)
	if err != nil {
		return []byte{}
	}
	return token
}

// SimpleScope is a simple struct to hold the scope data
type SimpleScope struct {
	ServiceName    string
	LibraryName    string
	LibraryVersion string
	ScopeSpans     ptrace.SpanSlice
	ScopeLogs      plog.LogRecordSlice
}

// ScopeHistory is a map of scope keys to the SimpleScope object
type ScopeHistory struct {
	Scope map[string]SimpleScope // key here is service.name+library.name
}

// ServiceHistory is a map of service names to the number of times they've been seen
type ServiceHistory struct {
	NameCount map[string]int
}

// ToPLogRecord converts a LibhoneyEvent to a Pdata LogRecord
func (l *LibhoneyEvent) ToPLogRecord(newLog *plog.LogRecord, alreadyUsedFields *[]string, logger zap.Logger) error {
	timeNs := l.MsgPackTimestamp.UnixNano()
	logger.Debug("processing log with", zap.Int64("timestamp", timeNs))
	newLog.SetTimestamp(pcommon.Timestamp(timeNs))

	if logSevCode, ok := l.Data["severity_code"]; ok {
		logSevInt := int32(logSevCode.(int64))
		newLog.SetSeverityNumber(plog.SeverityNumber(logSevInt))
	}

	if logSevText, ok := l.Data["severity_text"]; ok {
		newLog.SetSeverityText(logSevText.(string))
	}

	if logFlags, ok := l.Data["flags"]; ok {
		logFlagsUint := uint32(logFlags.(uint64))
		newLog.SetFlags(plog.LogRecordFlags(logFlagsUint))
	}

	// undoing this is gonna be complicated: https://github.com/honeycombio/husky/blob/91c0498333cd9f5eed1fdb8544ca486db7dea565/otlp/logs.go#L61
	if logBody, ok := l.Data["body"]; ok {
		newLog.Body().SetStr(logBody.(string))
	}

	newLog.Attributes().PutInt("SampleRate", int64(l.Samplerate))

	logFieldsAlready := []string{"severity_text", "severity_code", "flags", "body"}
	for k, v := range l.Data {
		if slices.Contains(*alreadyUsedFields, k) {
			continue
		}
		if slices.Contains(logFieldsAlready, k) {
			continue
		}
		switch v := v.(type) {
		case string:
			newLog.Attributes().PutStr(k, v)
		case int:
			newLog.Attributes().PutInt(k, int64(v))
		case int64, int16, int32:
			intv := v.(int64)
			newLog.Attributes().PutInt(k, intv)
		case float64:
			newLog.Attributes().PutDouble(k, v)
		case bool:
			newLog.Attributes().PutBool(k, v)
		default:
			logger.Warn("Span data type issue", zap.Int64("timestamp", timeNs), zap.String("key", k))
		}
	}
	return nil
}

// GetParentID returns the parent id from the event or an error if it's not found
func (l *LibhoneyEvent) GetParentID(fieldName string) (trc.SpanID, error) {
	if pid, ok := l.Data[fieldName]; ok {
		pid := strings.ReplaceAll(pid.(string), "-", "")
		pidByteArray, err := hex.DecodeString(pid)
		if err == nil {
			if len(pidByteArray) == 32 {
				pidByteArray = pidByteArray[8:24]
			} else if len(pidByteArray) >= 16 {
				pidByteArray = pidByteArray[0:16]
			}
			return trc.SpanID(pidByteArray), nil
		}
		return trc.SpanID{}, errors.New("parent id is not a valid span id")
	}
	return trc.SpanID{}, errors.New("parent id not found")
}

// ToPTraceSpan converts a LibhoneyEvent to a Pdata Span
func (l *LibhoneyEvent) ToPTraceSpan(newSpan *ptrace.Span, alreadyUsedFields *[]string, cfg FieldMapConfig, logger zap.Logger) error {
	timeNs := l.MsgPackTimestamp.UnixNano()
	logger.Debug("processing trace with", zap.Int64("timestamp", timeNs))

	var parentID trc.SpanID
	if pid, ok := l.Data[cfg.Attributes.ParentID]; ok {
		parentID = spanIDFrom(pid.(string))
		newSpan.SetParentSpanID(pcommon.SpanID(parentID))
	}

	durationMs := 0.0
	for _, df := range cfg.Attributes.DurationFields {
		if duration, okay := l.Data[df]; okay {
			durationMs = duration.(float64)
			break
		}
	}
	endTimestamp := timeNs + (int64(durationMs) * 1000000)

	if tid, ok := l.Data[cfg.Attributes.TraceID]; ok {
		tid := strings.ReplaceAll(tid.(string), "-", "")
		tidByteArray, err := hex.DecodeString(tid)
		if err == nil {
			if len(tidByteArray) >= 32 {
				tidByteArray = tidByteArray[0:32]
			}
			newSpan.SetTraceID(pcommon.TraceID(tidByteArray))
		} else {
			newSpan.SetTraceID(pcommon.TraceID(traceIDFrom(tid)))
		}
	} else {
		newSpan.SetTraceID(pcommon.TraceID(generateAnID(32)))
	}

	if sid, ok := l.Data[cfg.Attributes.SpanID]; ok {
		sid := strings.ReplaceAll(sid.(string), "-", "")
		sidByteArray, err := hex.DecodeString(sid)
		if err == nil {
			if len(sidByteArray) == 32 {
				sidByteArray = sidByteArray[8:24]
			} else if len(sidByteArray) >= 16 {
				sidByteArray = sidByteArray[0:16]
			}
			newSpan.SetSpanID(pcommon.SpanID(sidByteArray))
		} else {
			newSpan.SetSpanID(pcommon.SpanID(spanIDFrom(sid)))
		}
	} else {
		newSpan.SetSpanID(pcommon.SpanID(generateAnID(16)))
	}

	newSpan.SetStartTimestamp(pcommon.Timestamp(timeNs))
	newSpan.SetEndTimestamp(pcommon.Timestamp(endTimestamp))

	if spanName, ok := l.Data[cfg.Attributes.Name]; ok {
		newSpan.SetName(spanName.(string))
	}
	if spanStatusMessage, ok := l.Data["status_message"]; ok {
		newSpan.Status().SetMessage(spanStatusMessage.(string))
	}
	newSpan.Status().SetCode(ptrace.StatusCodeUnset)

	if _, ok := l.Data[cfg.Attributes.Error]; ok {
		newSpan.Status().SetCode(ptrace.StatusCodeError)
	}

	if spanKind, ok := l.Data[cfg.Attributes.SpanKind]; ok {
		switch spanKind.(string) {
		case "server":
			newSpan.SetKind(ptrace.SpanKindServer)
		case "client":
			newSpan.SetKind(ptrace.SpanKindClient)
		case "producer":
			newSpan.SetKind(ptrace.SpanKindProducer)
		case "consumer":
			newSpan.SetKind(ptrace.SpanKindConsumer)
		case "internal":
			newSpan.SetKind(ptrace.SpanKindInternal)
		default:
			newSpan.SetKind(ptrace.SpanKindUnspecified)
		}
	}

	newSpan.Attributes().PutInt("SampleRate", int64(l.Samplerate))

	for k, v := range l.Data {
		if slices.Contains(*alreadyUsedFields, k) {
			continue
		}
		switch v := v.(type) {
		case string:
			newSpan.Attributes().PutStr(k, v)
		case int:
			newSpan.Attributes().PutInt(k, int64(v))
		case int64, int16, int32:
			intv := v.(int64)
			newSpan.Attributes().PutInt(k, intv)
		case float64:
			newSpan.Attributes().PutDouble(k, v)
		case bool:
			newSpan.Attributes().PutBool(k, v)
		default:
			logger.Warn("Span data type issue", zap.String("trace.trace_id", newSpan.TraceID().String()), zap.String("trace.span_id", newSpan.SpanID().String()), zap.String("key", k))
		}
	}
	return nil
}
