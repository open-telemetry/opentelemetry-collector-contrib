// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logfmt/logfmt"
	faroTypes "github.com/grafana/faro/pkg/go"
	"github.com/zeebo/xxh3"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.uber.org/multierr"
)

type kvTime struct {
	kv    *keyVal
	ts    time.Time
	kind  faroTypes.Kind
	hash  uint64
	trace faroTypes.TraceContext
}

// TranslateToLogs converts faro.Payload into Logs pipeline data
func TranslateToLogs(ctx context.Context, payload faroTypes.Payload) (plog.Logs, error) {
	logs := plog.NewLogs()
	_, span := otel.Tracer("").Start(ctx, "TranslateToLogs")
	defer span.End()
	var kvList []*kvTime

	for _, logItem := range payload.Logs {
		kvList = append(kvList, &kvTime{
			kv:    logToKeyVal(logItem),
			ts:    logItem.Timestamp,
			kind:  faroTypes.KindLog,
			trace: logItem.Trace,
		})
	}
	for _, exception := range payload.Exceptions {
		kvList = append(kvList, &kvTime{
			kv:    exceptionToKeyVal(exception),
			ts:    exception.Timestamp,
			kind:  faroTypes.KindException,
			hash:  xxh3.HashString(exception.Value),
			trace: exception.Trace,
		})
	}
	for _, measurement := range payload.Measurements {
		kvList = append(kvList, &kvTime{
			kv:    measurementToKeyVal(measurement),
			ts:    measurement.Timestamp,
			kind:  faroTypes.KindMeasurement,
			trace: measurement.Trace,
		})
	}
	for _, event := range payload.Events {
		kvList = append(kvList, &kvTime{
			kv:    eventToKeyVal(event),
			ts:    event.Timestamp,
			kind:  faroTypes.KindEvent,
			trace: event.Trace,
		})
	}
	span.SetAttributes(attribute.Int("count", len(kvList)))
	if len(kvList) == 0 {
		return logs, nil
	}

	meta := metaToKeyVal(payload.Meta)
	rls := logs.ResourceLogs().AppendEmpty()
	rls.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), payload.Meta.App.Name)
	rls.Resource().Attributes().PutStr(string(semconv.ServiceVersionKey), payload.Meta.App.Version)
	rls.Resource().Attributes().PutStr(string(semconv.DeploymentEnvironmentKey), payload.Meta.App.Environment)
	if payload.Meta.App.Namespace != "" {
		rls.Resource().Attributes().PutStr(string(semconv.ServiceNamespaceKey), payload.Meta.App.Namespace)
	}
	if payload.Meta.App.BundleID != "" {
		rls.Resource().Attributes().PutStr(faroAppBundleID, payload.Meta.App.BundleID)
	}

	sl := rls.ScopeLogs().AppendEmpty()
	var errs error
	for _, i := range kvList {
		mergeKeyVal(i.kv, meta)
		line, err := logfmt.MarshalKeyvals(keyValToInterfaceSlice(i.kv)...)
		// If there is an error, we skip the log record
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.Body().SetStr(string(line))
		logRecord.Attributes().PutStr(faroKind, string(i.kind))
		if (i.kind == faroTypes.KindException) && (i.hash != 0) {
			logRecord.Attributes().PutStr(faroExceptionHash, strconv.FormatUint(i.hash, 10))
		}
		observedTimestamp := pcommon.NewTimestampFromTime(time.Now())
		logRecord.SetObservedTimestamp(observedTimestamp)
		if !i.ts.IsZero() {
			logRecord.SetTimestamp(pcommon.NewTimestampFromTime(i.ts))
		}

		spanID := i.trace.SpanID
		if spanID != "" {
			var spanIDBytes [8]byte
			byteSlice, err := hex.DecodeString(spanID)
			// If there is an error, we skip the log record
			if err != nil {
				errs = multierr.Append(errs, fmt.Errorf("failed to decode spanID=%s: %w", spanID, err))
				continue
			}
			copy(spanIDBytes[:], byteSlice)
			logRecord.SetSpanID(spanIDBytes)
		}

		traceID := i.trace.TraceID
		if traceID != "" {
			var traceIDBytes [16]byte
			byteSlice, err := hex.DecodeString(traceID)
			// If there is an error, we skip the log record
			if err != nil {
				errs = multierr.Append(errs, fmt.Errorf("failed to decode traceID=%s: %w", traceID, err))
				continue
			}
			copy(traceIDBytes[:], byteSlice)
			logRecord.SetTraceID(traceIDBytes)
		}
	}
	return logs, errs
}
