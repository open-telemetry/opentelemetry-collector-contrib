// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parser // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/parser"

import (
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	trc "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/libhoneyevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/response"
)

// IndexMapping tracks which original libhoney event indices became logs vs traces
type IndexMapping struct {
	LogIndices   []int // Original event indices that became logs
	TraceIndices []int // Original event indices that became traces/spans/span events
}

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

// ToPdata converts a list of LibhoneyEvents to a Pdata Logs object and tracks which original indices became what
func ToPdata(dataset string, lhes []libhoneyevent.LibhoneyEvent, cfg libhoneyevent.FieldMapConfig, logger zap.Logger) (plog.Logs, ptrace.Traces, IndexMapping, []response.ResponseInBatch) {
	foundServices := libhoneyevent.ServiceHistory{}
	foundServices.NameCount = make(map[string]int)
	foundScopes := libhoneyevent.ScopeHistory{}
	foundScopes.Scope = make(map[string]libhoneyevent.SimpleScope)

	spanLinks := map[trc.SpanID][]libhoneyevent.LibhoneyEvent{}
	spanEvents := map[trc.SpanID][]libhoneyevent.LibhoneyEvent{}

	// Initialize index mapping to track which original events become logs vs traces
	indexMapping := IndexMapping{
		LogIndices:   make([]int, 0),
		TraceIndices: make([]int, 0),
	}

	// Initialize parsing results to track success/failure per original event
	parsingResults := make([]response.ResponseInBatch, len(lhes))

	foundScopes.Scope = make(map[string]libhoneyevent.SimpleScope) // a list of already seen scopes

	alreadyUsedFields := []string{cfg.Resources.ServiceName, cfg.Scopes.LibraryName, cfg.Scopes.LibraryVersion}
	alreadyUsedTraceFields := []string{
		cfg.Attributes.Name,
		cfg.Attributes.TraceID, cfg.Attributes.ParentID, cfg.Attributes.SpanID,
		cfg.Attributes.Error, cfg.Attributes.SpanKind,
	}

	for i, lhe := range lhes {
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
				parsingResults[i] = response.ResponseInBatch{
					Status:   http.StatusBadRequest,
					ErrorStr: "span parsing failed: " + err.Error(),
				}
			} else {
				// Track successful span for consumer processing
				indexMapping.TraceIndices = append(indexMapping.TraceIndices, i)
				parsingResults[i] = response.ResponseInBatch{
					Status: http.StatusAccepted,
				}
			}
		case "log":
			logService, _ := lhe.GetService(cfg, &foundServices, dataset)
			logScopeKey, _ := lhe.GetScope(cfg, &foundScopes, logService) // adds a new found scope if needed
			newLog := foundScopes.Scope[logScopeKey].ScopeLogs.AppendEmpty()
			err := lhe.ToPLogRecord(&newLog, &alreadyUsedFields, logger)
			if err != nil {
				logger.Warn("log could not be converted from libhoney to plog", zap.String("span.object", lhe.DebugString()))
				parsingResults[i] = response.ResponseInBatch{
					Status:   http.StatusBadRequest,
					ErrorStr: "log parsing failed: " + err.Error(),
				}
			} else {
				// Track successful log for consumer processing
				indexMapping.LogIndices = append(indexMapping.LogIndices, i)
				parsingResults[i] = response.ResponseInBatch{
					Status: http.StatusAccepted,
				}
			}
		case "span_event":
			// Span events are processed later, so we need index mapping for them
			indexMapping.TraceIndices = append(indexMapping.TraceIndices, i)
			spanEvents[parentID] = append(spanEvents[parentID], lhe)
		case "span_link":
			// Span links are processed later, so we need index mapping for them
			indexMapping.TraceIndices = append(indexMapping.TraceIndices, i)
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
			lr.SetSchemaUrl(conventions.SchemaURL)
			lr.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), ss.ServiceName)

			ls := lr.ScopeLogs().AppendEmpty()
			ls.Scope().SetName(ss.LibraryName)
			ls.Scope().SetVersion(ss.LibraryVersion)
			foundScopes.Scope[scopeName].ScopeLogs.MoveAndAppendTo(ls.LogRecords())
		}
		if ss.ScopeSpans.Len() > 0 {
			tr := resultTraces.ResourceSpans().AppendEmpty()
			tr.SetSchemaUrl(conventions.SchemaURL)
			tr.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), ss.ServiceName)

			ts := tr.ScopeSpans().AppendEmpty()
			ts.Scope().SetName(ss.LibraryName)
			ts.Scope().SetVersion(ss.LibraryVersion)
			foundScopes.Scope[scopeName].ScopeSpans.MoveAndAppendTo(ts.Spans())
		}
	}

	return resultLogs, resultTraces, indexMapping, parsingResults
}

func addSpanEventsToSpan(sp ptrace.Span, events []libhoneyevent.LibhoneyEvent, alreadyUsedFields []string, logger *zap.Logger) {
	for _, spe := range events {
		newEvent := sp.Events().AppendEmpty()
		// Handle cases where MsgPackTimestamp might be nil (e.g., JSON data from Refinery)
		if spe.MsgPackTimestamp != nil {
			newEvent.SetTimestamp(pcommon.Timestamp(spe.MsgPackTimestamp.UnixNano()))
		} else {
			// Use current time if timestamp is not available
			newEvent.SetTimestamp(pcommon.Timestamp(time.Now().UnixNano()))
		}
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
			if len(tidByteArray) != 16 {
				logger.Debug("span link trace ID wrong length",
					zap.Int("length", len(tidByteArray)),
					zap.String("span link contents", spl.DebugString()))
				continue
			}
			// Convert slice to [16]byte array
			var traceIDArray [16]byte
			copy(traceIDArray[:], tidByteArray)
			newLink.SetTraceID(pcommon.TraceID(traceIDArray))
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
			if len(sidByteArray) != 8 {
				logger.Debug("span link span ID wrong length",
					zap.Int("length", len(sidByteArray)),
					zap.String("span link contents", spl.DebugString()))
				continue
			}
			// Convert slice to [8]byte array
			var spanIDArray [8]byte
			copy(spanIDArray[:], sidByteArray)
			newLink.SetSpanID(pcommon.SpanID(spanIDArray))
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
