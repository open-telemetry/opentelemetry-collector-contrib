// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
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
			hash: xxh3.HashString(exception.Value),
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

	meta := MetaToKeyVal(payload.Meta)
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
