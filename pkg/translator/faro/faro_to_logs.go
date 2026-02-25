// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/go-logfmt/logfmt"
	faroTypes "github.com/grafana/faro/pkg/go"
	"github.com/zeebo/xxh3"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	conventionsv126 "go.opentelemetry.io/otel/semconv/v1.26.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/multierr"
)

type kvTime struct {
	kv    *keyVal
	ts    time.Time
	kind  faroTypes.Kind
	hash  uint64
	trace faroTypes.TraceContext
}

var (
	// Compile regex patterns once for performance
	propertyAccessRegex = regexp.MustCompile(`Cannot read (property|properties) '([^']+)'`)
	methodCallRegex     = regexp.MustCompile(`Cannot read (property|properties) '([^']+)' of`)
	urlRegex            = regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)
	memoryAddressRegex  = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	uuidRegex           = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	numericIDRegex      = regexp.MustCompile(`\b(id|ID|Id)\s*[:\s=]\s*\d+\b`)
	timestampRegex      = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`)
	filePathRegex       = regexp.MustCompile(`(?:[A-Za-z]:)?[/\\][\w\-._/\\]+\.(js|ts|jsx|tsx|css|html)\b`)
)

// drainExceptionValue normalizes exception values by replacing instance-specific
// identifiers with placeholders to improve exception grouping by hash.
func drainExceptionValue(value string) string {
	// Replace property access patterns
	drained := propertyAccessRegex.ReplaceAllString(value, "Cannot read $1 '<PROPERTY>'")
	drained = methodCallRegex.ReplaceAllString(drained, "Cannot read $1 '<PROPERTY>' of")

	// Replace URLs first (takes precedence over file paths)
	drained = urlRegex.ReplaceAllString(drained, "<URL>")

	// Replace memory addresses
	drained = memoryAddressRegex.ReplaceAllString(drained, "<ADDRESS>")

	// Replace UUIDs
	drained = uuidRegex.ReplaceAllString(drained, "<UUID>")

	// Replace numeric IDs
	drained = numericIDRegex.ReplaceAllString(drained, "${1} <ID>")

	// Replace timestamps
	drained = timestampRegex.ReplaceAllString(drained, "<TIMESTAMP>")

	// Replace file paths (after URLs to avoid conflicts)
	drained = filePathRegex.ReplaceAllString(drained, "<PATH>")

	return drained
}

// TranslateToLogs converts faro.Payload into Logs pipeline data
func TranslateToLogs(ctx context.Context, payload faroTypes.Payload) (plog.Logs, error) {
	logs := plog.NewLogs()
	_, span := otel.Tracer("").Start(ctx, "TranslateToLogs")
	defer span.End()
	var kvList []*kvTime

	for i := range payload.Logs {
		logItem := &payload.Logs[i]
		kvList = append(kvList, &kvTime{
			kv:    logToKeyVal(logItem),
			ts:    logItem.Timestamp,
			kind:  faroTypes.KindLog,
			trace: logItem.Trace,
		})
	}
	for i := range payload.Exceptions {
		exception := &payload.Exceptions[i]
		kvList = append(kvList, &kvTime{
			kv:    exceptionToKeyVal(exception),
			ts:    exception.Timestamp,
			kind:  faroTypes.KindException,
			hash:  xxh3.HashString(drainExceptionValue(exception.Value)),
			trace: exception.Trace,
		})
	}
	for i := range payload.Measurements {
		measurement := &payload.Measurements[i]
		kvList = append(kvList, &kvTime{
			kv:    measurementToKeyVal(measurement),
			ts:    measurement.Timestamp,
			kind:  faroTypes.KindMeasurement,
			trace: measurement.Trace,
		})
	}
	for i := range payload.Events {
		event := &payload.Events[i]
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
	rls.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), payload.Meta.App.Name)
	rls.Resource().Attributes().PutStr(string(conventions.ServiceVersionKey), payload.Meta.App.Version)
	rls.Resource().Attributes().PutStr(string(conventionsv126.DeploymentEnvironmentKey), payload.Meta.App.Environment)
	if payload.Meta.App.Namespace != "" {
		rls.Resource().Attributes().PutStr(string(conventions.ServiceNamespaceKey), payload.Meta.App.Namespace)
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
