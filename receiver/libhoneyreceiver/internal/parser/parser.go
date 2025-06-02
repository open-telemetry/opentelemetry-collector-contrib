// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parser // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/parser"

import (
	"encoding/hex"
	"errors"
	"net/url"
	"slices"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.16.0"
	trc "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/libhoneyevent"
)

// GetDatasetFromRequest extracts the dataset name from the request path
func GetDatasetFromRequest(path string) (string, error) {
	if path == "" {
		return "", errors.New("missing dataset name")
	}
	dataset, err := url.PathUnescape(path)
	if err != nil {
		return "", err
	}
	return dataset, nil
}

// ToPdata converts a list of LibhoneyEvents to a Pdata Logs object
func ToPdata(dataset string, lhes []libhoneyevent.LibhoneyEvent, cfg libhoneyevent.FieldMapConfig, logger zap.Logger) (plog.Logs, ptrace.Traces) {
	foundServices := libhoneyevent.ServiceHistory{}
	foundServices.NameCount = make(map[string]int)
	foundScopes := libhoneyevent.ScopeHistory{}
	foundScopes.Scope = make(map[string]libhoneyevent.SimpleScope)

	spanLinks := map[trc.SpanID][]libhoneyevent.LibhoneyEvent{}
	spanEvents := map[trc.SpanID][]libhoneyevent.LibhoneyEvent{}

	foundScopes.Scope = make(map[string]libhoneyevent.SimpleScope) // a list of already seen scopes
	foundScopes.Scope["libhoney.receiver"] = libhoneyevent.SimpleScope{
		ServiceName:    dataset,
		LibraryName:    "libhoney.receiver",
		LibraryVersion: "1.0.0",
		ScopeSpans:     ptrace.NewSpanSlice(),
		ScopeLogs:      plog.NewLogRecordSlice(),
	} // seed a default

	alreadyUsedFields := []string{cfg.Resources.ServiceName, cfg.Scopes.LibraryName, cfg.Scopes.LibraryVersion}
	alreadyUsedTraceFields := []string{
		cfg.Attributes.Name,
		cfg.Attributes.TraceID, cfg.Attributes.ParentID, cfg.Attributes.SpanID,
		cfg.Attributes.Error, cfg.Attributes.SpanKind,
	}

	for _, lhe := range lhes {
		parentID, err := lhe.GetParentID(cfg.Attributes.ParentID)
		if err != nil {
			logger.Debug("parent id not found")
		}

		action := lhe.SignalType(logger)
		switch action {
		case "span":
			spanService, _ := lhe.GetService(cfg, &foundServices, dataset)
			spanScopeKey, _ := lhe.GetScope(cfg, &foundScopes, spanService) // adds a new found scope if needed
			newSpan := foundScopes.Scope[spanScopeKey].ScopeSpans.AppendEmpty()
			alreadyUsedFields = append(alreadyUsedFields, alreadyUsedTraceFields...)
			alreadyUsedFields = append(alreadyUsedFields, cfg.Attributes.DurationFields...)
			err := lhe.ToPTraceSpan(&newSpan, &alreadyUsedFields, cfg, logger)
			if err != nil {
				logger.Warn("span could not be converted from libhoney to ptrace", zap.String("span.object", lhe.DebugString()))
			}
		case "log":
			logService, _ := lhe.GetService(cfg, &foundServices, dataset)
			logScopeKey, _ := lhe.GetScope(cfg, &foundScopes, logService) // adds a new found scope if needed
			newLog := foundScopes.Scope[logScopeKey].ScopeLogs.AppendEmpty()
			err := lhe.ToPLogRecord(&newLog, &alreadyUsedFields, logger)
			if err != nil {
				logger.Warn("log could not be converted from libhoney to plog", zap.String("span.object", lhe.DebugString()))
			}
		case "span_event":
			spanEvents[parentID] = append(spanEvents[parentID], lhe)
		case "span_link":
			spanLinks[parentID] = append(spanLinks[parentID], lhe)
		}
	}

	start := time.Now()
	for _, ss := range foundScopes.Scope {
		for i := 0; i < ss.ScopeSpans.Len(); i++ {
			sp := ss.ScopeSpans.At(i)
			spID := trc.SpanID(sp.SpanID())

			if speArr, ok := spanEvents[spID]; ok {
				addSpanEventsToSpan(sp, speArr, alreadyUsedFields, &logger)
			}

			if splArr, ok := spanLinks[spID]; ok {
				addSpanLinksToSpan(sp, splArr, alreadyUsedFields, &logger)
			}
		}
	}
	logger.Debug("time to reattach span events and links", zap.Duration("duration", time.Since(start)))

	resultLogs := plog.NewLogs()
	resultTraces := ptrace.NewTraces()

	for scopeName, ss := range foundScopes.Scope {
		if ss.ScopeLogs.Len() > 0 {
			lr := resultLogs.ResourceLogs().AppendEmpty()
			lr.SetSchemaUrl(semconv.SchemaURL)
			lr.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), ss.ServiceName)

			ls := lr.ScopeLogs().AppendEmpty()
			ls.Scope().SetName(ss.LibraryName)
			ls.Scope().SetVersion(ss.LibraryVersion)
			foundScopes.Scope[scopeName].ScopeLogs.MoveAndAppendTo(ls.LogRecords())
		}
		if ss.ScopeSpans.Len() > 0 {
			tr := resultTraces.ResourceSpans().AppendEmpty()
			tr.SetSchemaUrl(semconv.SchemaURL)
			tr.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), ss.ServiceName)

			ts := tr.ScopeSpans().AppendEmpty()
			ts.Scope().SetName(ss.LibraryName)
			ts.Scope().SetVersion(ss.LibraryVersion)
			foundScopes.Scope[scopeName].ScopeSpans.MoveAndAppendTo(ts.Spans())
		}
	}

	return resultLogs, resultTraces
}

func addSpanEventsToSpan(sp ptrace.Span, events []libhoneyevent.LibhoneyEvent, alreadyUsedFields []string, logger *zap.Logger) {
	for _, spe := range events {
		newEvent := sp.Events().AppendEmpty()
		newEvent.SetTimestamp(pcommon.Timestamp(spe.MsgPackTimestamp.UnixNano()))
		newEvent.SetName(spe.Data["name"].(string))
		for lkey, lval := range spe.Data {
			if slices.Contains(alreadyUsedFields, lkey) {
				continue
			}
			if lkey == "meta.annotation_type" || lkey == "meta.signal_type" {
				continue
			}
			switch lval := lval.(type) {
			case string:
				newEvent.Attributes().PutStr(lkey, lval)
			case int:
				newEvent.Attributes().PutInt(lkey, int64(lval))
			case int64, int16, int32:
				intv := lval.(int64)
				newEvent.Attributes().PutInt(lkey, intv)
			case float64:
				newEvent.Attributes().PutDouble(lkey, lval)
			case bool:
				newEvent.Attributes().PutBool(lkey, lval)
			default:
				logger.Debug("SpanEvent data type issue",
					zap.String("trace.trace_id", sp.TraceID().String()),
					zap.String("trace.span_id", sp.SpanID().String()),
					zap.String("key", lkey))
			}
		}
	}
}

func addSpanLinksToSpan(sp ptrace.Span, links []libhoneyevent.LibhoneyEvent, alreadyUsedFields []string, logger *zap.Logger) {
	for _, spl := range links {
		newLink := sp.Links().AppendEmpty()

		if linkTraceStr, ok := spl.Data["trace.link.trace_id"]; ok {
			tidByteArray, err := hex.DecodeString(linkTraceStr.(string))
			if err != nil {
				logger.Debug("span link invalid",
					zap.String("missing.attribute", "trace.link.trace_id"),
					zap.String("span link contents", spl.DebugString()))
				continue
			}
			if len(tidByteArray) >= 32 {
				tidByteArray = tidByteArray[0:32]
			}
			newLink.SetTraceID(pcommon.TraceID(tidByteArray))
		} else {
			logger.Debug("span link missing attributes",
				zap.String("missing.attribute", "trace.link.trace_id"),
				zap.String("span link contents", spl.DebugString()))
			continue
		}

		if linkSpanStr, ok := spl.Data["trace.link.span_id"]; ok {
			sidByteArray, err := hex.DecodeString(linkSpanStr.(string))
			if err != nil {
				logger.Debug("span link invalid",
					zap.String("missing.attribute", "trace.link.span_id"),
					zap.String("span link contents", spl.DebugString()))
				continue
			}
			if len(sidByteArray) >= 16 {
				sidByteArray = sidByteArray[0:16]
			}
			newLink.SetSpanID(pcommon.SpanID(sidByteArray))
		} else {
			logger.Debug("span link missing attributes",
				zap.String("missing.attribute", "trace.link.span_id"),
				zap.String("span link contents", spl.DebugString()))
			continue
		}

		for lkey, lval := range spl.Data {
			if len(lkey) > 10 && lkey[:11] == "trace.link." {
				continue
			}
			if slices.Contains(alreadyUsedFields, lkey) {
				continue
			}
			if lkey == "meta.annotation_type" || lkey == "meta.signal_type" {
				continue
			}
			switch lval := lval.(type) {
			case string:
				newLink.Attributes().PutStr(lkey, lval)
			case int:
				newLink.Attributes().PutInt(lkey, int64(lval))
			case int64, int16, int32:
				intv := lval.(int64)
				newLink.Attributes().PutInt(lkey, intv)
			case float64:
				newLink.Attributes().PutDouble(lkey, lval)
			case bool:
				newLink.Attributes().PutBool(lkey, lval)
			default:
				logger.Debug("SpanLink data type issue",
					zap.String("trace.trace_id", sp.TraceID().String()),
					zap.String("trace.span_id", sp.SpanID().String()),
					zap.String("key", lkey))
			}
		}
	}
}
