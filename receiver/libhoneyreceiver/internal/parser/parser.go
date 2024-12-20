// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parser // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/parser"

import (
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/libhoneyevent"
)

// GetDatasetFromRequest extracts the dataset name from the request path
func GetDatasetFromRequest(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("missing dataset name")
	}
	dataset, err := url.PathUnescape(path)
	if err != nil {
		return "", err
	}
	return dataset, nil
}

// ToPdata converts a list of LibhoneyEvents to a Pdata Logs object
func ToPdata(dataset string, lhes []libhoneyevent.LibhoneyEvent, cfg libhoneyevent.FieldMapConfig, logger zap.Logger) plog.Logs {
	foundServices := libhoneyevent.ServiceHistory{}
	foundServices.NameCount = make(map[string]int)
	foundScopes := libhoneyevent.ScopeHistory{}
	foundScopes.Scope = make(map[string]libhoneyevent.SimpleScope)

	foundScopes.Scope = make(map[string]libhoneyevent.SimpleScope) // a list of already seen scopes
	foundScopes.Scope["libhoney.receiver"] = libhoneyevent.SimpleScope{
		ServiceName:    dataset,
		LibraryName:    "libhoney.receiver",
		LibraryVersion: "1.0.0",
		ScopeSpans:     ptrace.NewSpanSlice(),
		ScopeLogs:      plog.NewLogRecordSlice(),
	} // seed a default

	alreadyUsedFields := []string{cfg.Resources.ServiceName, cfg.Scopes.LibraryName, cfg.Scopes.LibraryVersion}
	alreadyUsedFields = append(alreadyUsedFields, cfg.Attributes.Name,
		cfg.Attributes.TraceID, cfg.Attributes.ParentID, cfg.Attributes.SpanID,
		cfg.Attributes.Error, cfg.Attributes.SpanKind,
	)
	alreadyUsedFields = append(alreadyUsedFields, cfg.Attributes.DurationFields...)

	for _, lhe := range lhes {
		action, err := lhe.SignalType()
		if err != nil {
			logger.Warn("signal type unclear")
		}
		switch action {
		case "span":
			// not implemented
		case "log":
			logService, _ := lhe.GetService(cfg, &foundServices, dataset)
			logScopeKey, _ := lhe.GetScope(cfg, &foundScopes, logService) // adds a new found scope if needed
			newLog := foundScopes.Scope[logScopeKey].ScopeLogs.AppendEmpty()
			err := lhe.ToPLogRecord(&newLog, &alreadyUsedFields, logger)
			if err != nil {
				logger.Warn("log could not be converted from libhoney to plog", zap.String("span.object", lhe.DebugString()))
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
