// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver"

import (
	"fmt"
	"mime"
	"net/http"
	"net/url"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/simplespan"
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

func toPsomething(dataset string, ss []simplespan.SimpleSpan, cfg Config, logger zap.Logger) plog.Logs {
	foundServices := simplespan.ServiceHistory{}
	foundServices.NameCount = make(map[string]int)
	foundScopes := simplespan.ScopeHistory{}
	foundScopes.Scope = make(map[string]simplespan.SimpleScope)

	foundScopes.Scope = make(map[string]simplespan.SimpleScope) // a list of already seen scopes
	foundScopes.Scope["libhoney.receiver"] = simplespan.SimpleScope{
		ServiceName:    dataset,
		LibraryName:    "libhoney.receiver",
		LibraryVersion: "1.0.0",
		ScopeSpans:     ptrace.NewSpanSlice(),
		ScopeLogs:      plog.NewLogRecordSlice(),
	} // seed a default

	already_used_fields := []string{cfg.FieldMapConfig.Resources.ServiceName, cfg.FieldMapConfig.Scopes.LibraryName, cfg.FieldMapConfig.Scopes.LibraryVersion}
	already_used_fields = append(already_used_fields, cfg.FieldMapConfig.Attributes.Name,
		cfg.FieldMapConfig.Attributes.TraceID, cfg.FieldMapConfig.Attributes.ParentID, cfg.FieldMapConfig.Attributes.SpanID,
		cfg.FieldMapConfig.Attributes.Error, cfg.FieldMapConfig.Attributes.SpanKind,
	)
	already_used_fields = append(already_used_fields, cfg.FieldMapConfig.Attributes.DurationFields...)

	for _, span := range ss {
		action, err := span.SignalType()
		if err != nil {
			logger.Warn("signal type unclear")
		}
		switch action {
		case "span":
			// not implemented
		case "log":
			logService, _ := span.GetService(cfg.FieldMapConfig, &foundServices, dataset)
			logScopeKey, _ := span.GetScope(cfg.FieldMapConfig, &foundScopes, logService) // adds a new found scope if needed
			newLog := foundScopes.Scope[logScopeKey].ScopeLogs.AppendEmpty()
			err := span.ToPLogRecord(&newLog, &already_used_fields, logger)
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

	return resultLogs
}
