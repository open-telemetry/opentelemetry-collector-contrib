// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver"

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/eventtime"
)

func readContentType(resp http.ResponseWriter, req *http.Request) (encoder, bool) {
	if req.Method != http.MethodPost {
		handleUnmatchedMethod(resp)
		return nil, false
	}

	switch getMimeTypeFromContentType(req.Header.Get("Content-Type")) {
	case jsonContentType:
		return jsEncoder, true
	case "application/x-msgpack", "application/msgpack":
		return mpEncoder, true
	default:
		handleUnmatchedContentType(resp)
		return nil, false
	}
}

func writeResponse(w http.ResponseWriter, contentType string, statusCode int, msg []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode)
	_, _ = w.Write(msg)
}

func getMimeTypeFromContentType(contentType string) string {
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return mediatype
}

func handleUnmatchedMethod(resp http.ResponseWriter) {
	status := http.StatusMethodNotAllowed
	writeResponse(resp, "text/plain", status, []byte(fmt.Sprintf("%v method not allowed, supported: [POST]", status)))
}

func handleUnmatchedContentType(resp http.ResponseWriter) {
	status := http.StatusUnsupportedMediaType
	writeResponse(resp, "text/plain", status, []byte(fmt.Sprintf("%v unsupported media type, supported: [%s, %s]", status, jsonContentType, pbContentType)))
}

// taken from refinery https://github.com/honeycombio/refinery/blob/v2.6.1/route/route.go#L964-L974
func getDatasetFromRequest(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("missing dataset name")
	}
	dataset, err := url.PathUnescape(path)
	if err != nil {
		return "", err
	}
	return dataset, nil
}

type simpleSpan struct {
	Samplerate       int                    `json:"samplerate" msgpack:"samplerate"`
	MsgPackTimestamp *time.Time             `msgpack:"time"`
	Time             string                 `json:"time"` // should not be trusted. use MsgPackTimestamp
	Data             map[string]interface{} `json:"data" msgpack:"data"`
}

// Overrides unmarshall to make sure the MsgPackTimestamp is set
func (s *simpleSpan) UnmarshalJSON(j []byte) error {
	type _simpleSpan simpleSpan
	tstr := eventtime.GetEventTimeDefaultString()
	tzero := time.Time{}
	tmp := _simpleSpan{Time: "none", MsgPackTimestamp: &tzero, Samplerate: 1}

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

	*s = simpleSpan(tmp)
	return nil
}

func (s *simpleSpan) DebugString() string {
	return fmt.Sprintf("%#v", s)
}

// returns log until we add the trace parser
func (s *simpleSpan) SignalType() (string, error) {
	return "log", nil
}

func (s *simpleSpan) GetService(cfg Config, seen *serviceHistory, dataset string) (string, error) {
	if serviceName, ok := s.Data[cfg.Resources.ServiceName]; ok {
		seen.NameCount[serviceName.(string)] += 1
		return serviceName.(string), nil
	}
	return dataset, errors.New("no service.name found in event")
}

func (s *simpleSpan) GetScope(cfg Config, seen *scopeHistory, serviceName string) (string, error) {
	if scopeLibraryName, ok := s.Data[cfg.Scopes.LibraryName]; ok {
		scopeKey := serviceName + scopeLibraryName.(string)
		if _, ok := seen.Scope[scopeKey]; ok {
			// if we've seen it, we don't expect it to be different right away so we'll just return it.
			return scopeKey, nil
		}
		// otherwise, we need to make a new found scope
		scopeLibraryVersion := "unset"
		if scopeLibVer, ok := s.Data[cfg.Scopes.LibraryVersion]; ok {
			scopeLibraryVersion = scopeLibVer.(string)
		}
		newScope := simpleScope{
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

type simpleScope struct {
	ServiceName    string
	LibraryName    string
	LibraryVersion string
	ScopeSpans     ptrace.SpanSlice
	ScopeLogs      plog.LogRecordSlice
}

type scopeHistory struct {
	Scope map[string]simpleScope // key here is service.name+library.name
}
type serviceHistory struct {
	NameCount map[string]int
}

func (s *simpleSpan) ToPLogRecord(newLog *plog.LogRecord, already_used_fields *[]string, cfg Config, logger zap.Logger) error {
	time_ns := s.MsgPackTimestamp.UnixNano()
	logger.Debug("processing log with", zap.Int64("timestamp", time_ns))
	newLog.SetTimestamp(pcommon.Timestamp(time_ns))

	if logSevCode, ok := s.Data["severity_code"]; ok {
		logSevInt := int32(logSevCode.(int64))
		newLog.SetSeverityNumber(plog.SeverityNumber(logSevInt))
	}

	if logSevText, ok := s.Data["severity_text"]; ok {
		newLog.SetSeverityText(logSevText.(string))
	}

	if logFlags, ok := s.Data["flags"]; ok {
		logFlagsUint := uint32(logFlags.(uint64))
		newLog.SetFlags(plog.LogRecordFlags(logFlagsUint))
	}

	// undoing this is gonna be complicated: https://github.com/honeycombio/husky/blob/91c0498333cd9f5eed1fdb8544ca486db7dea565/otlp/logs.go#L61
	if logBody, ok := s.Data["body"]; ok {
		newLog.Body().SetStr(logBody.(string))
	}

	newLog.Attributes().PutInt("SampleRate", int64(s.Samplerate))

	logFieldsAlready := []string{"severity_text", "severity_code", "flags", "body"}
	for k, v := range s.Data {
		if slices.Contains(*already_used_fields, k) {
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
			logger.Warn("Span data type issue", zap.Int64("timestamp", time_ns), zap.String("key", k))
		}
	}
	return nil
}

func toPsomething(dataset string, ss []simpleSpan, cfg Config, logger zap.Logger) (plog.Logs, error) {
	foundServices := serviceHistory{}
	foundServices.NameCount = make(map[string]int)
	foundScopes := scopeHistory{}
	foundScopes.Scope = make(map[string]simpleScope)

	foundScopes.Scope = make(map[string]simpleScope)                                                                                             // a list of already seen scopes
	foundScopes.Scope["libhoney.receiver"] = simpleScope{dataset, "libhoney.receiver", "1.0.0", ptrace.NewSpanSlice(), plog.NewLogRecordSlice()} // seed a default

	already_used_fields := []string{cfg.Resources.ServiceName, cfg.Scopes.LibraryName, cfg.Scopes.LibraryVersion}
	already_used_fields = append(already_used_fields, cfg.Attributes.Name,
		cfg.Attributes.TraceID, cfg.Attributes.ParentID, cfg.Attributes.SpanID,
		cfg.Attributes.Error, cfg.Attributes.SpanKind,
	)
	already_used_fields = append(already_used_fields, cfg.Attributes.DurationFields...)

	for _, span := range ss {
		action, err := span.SignalType()
		if err != nil {
			logger.Warn("signal type unclear")
		}
		switch action {
		case "span":
			// not implemented
		case "log":
			logService, _ := span.GetService(cfg, &foundServices, dataset)
			logScopeKey, _ := span.GetScope(cfg, &foundScopes, logService) // adds a new found scope if needed
			newLog := foundScopes.Scope[logScopeKey].ScopeLogs.AppendEmpty()
			span.ToPLogRecord(&newLog, &already_used_fields, cfg, logger)
			if err != nil {
				logger.Warn("log could not be converted from libhoney to plog", zap.String("span.object", span.DebugString()))
			}
		}
	}

	resultLogs := plog.NewLogs()

	for scopeName, ss := range foundScopes.Scope {
		if ss.ScopeLogs.Len() > 0 {
			lr := resultLogs.ResourceLogs().AppendEmpty()
			lr.SetSchemaUrl(semconv.SchemaURL)
			lr.Resource().Attributes().PutStr(semconv.AttributeServiceName, ss.ServiceName)

			ls := lr.ScopeLogs().AppendEmpty()
			ls.Scope().SetName(ss.LibraryName)
			ls.Scope().SetVersion(ss.LibraryVersion)
			foundScopes.Scope[scopeName].ScopeLogs.MoveAndAppendTo(ls.LogRecords())
		}
	}

	return resultLogs, nil
}
