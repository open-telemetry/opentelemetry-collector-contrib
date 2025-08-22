// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"github.com/go-logfmt/logfmt"
	faroTypes "github.com/grafana/faro/pkg/go"
	"github.com/zeebo/xxh3"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.uber.org/multierr"
)

type kvTime struct {
	kv   *keyVal
	ts   time.Time
	kind faroTypes.Kind
	hash uint64
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

	for _, logItem := range payload.Logs {
		kvList = append(kvList, &kvTime{
			kv:   logToKeyVal(logItem),
			ts:   logItem.Timestamp,
			kind: faroTypes.KindLog,
		})
	}
	for _, exception := range payload.Exceptions {
		kvList = append(kvList, &kvTime{
			kv:   exceptionToKeyVal(exception),
			ts:   exception.Timestamp,
			kind: faroTypes.KindException,
			hash: xxh3.HashString(drainExceptionValue(exception.Value)),
		})
	}
	for _, measurement := range payload.Measurements {
		kvList = append(kvList, &kvTime{
			kv:   measurementToKeyVal(measurement),
			ts:   measurement.Timestamp,
			kind: faroTypes.KindMeasurement,
		})
	}
	for _, event := range payload.Events {
		kvList = append(kvList, &kvTime{
			kv:   eventToKeyVal(event),
			ts:   event.Timestamp,
			kind: faroTypes.KindEvent,
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
	}
	return logs, errs
}
