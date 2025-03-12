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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type kvTime struct {
	kv   *KeyVal
	ts   time.Time
	kind faroTypes.Kind
	hash uint64
}

// TranslateToLogs converts faro.Payload into Logs pipeline data
func TranslateToLogs(ctx context.Context, payload faroTypes.Payload) (*plog.Logs, error) {
	_, span := otel.Tracer("").Start(ctx, "TranslateToLogs")
	defer span.End()
	var kvList []*kvTime

	for _, logItem := range payload.Logs {
		kvList = append(kvList, &kvTime{
			kv:   LogToKeyVal(logItem),
			ts:   logItem.Timestamp,
			kind: faroTypes.KindLog,
		})
	}
	for _, exception := range payload.Exceptions {
		kvList = append(kvList, &kvTime{
			kv:   ExceptionToKeyVal(exception),
			ts:   exception.Timestamp,
			kind: faroTypes.KindException,
			hash: xxh3.HashString(exception.Value),
		})
	}
	for _, measurement := range payload.Measurements {
		kvList = append(kvList, &kvTime{
			kv:   MeasurementToKeyVal(measurement),
			ts:   measurement.Timestamp,
			kind: faroTypes.KindMeasurement,
		})
	}
	for _, event := range payload.Events {
		kvList = append(kvList, &kvTime{
			kv:   EventToKeyVal(event),
			ts:   event.Timestamp,
			kind: faroTypes.KindEvent,
		})
	}
	if len(kvList) == 0 {
		return nil, nil
	}
	span.SetAttributes(attribute.Int("count", len(kvList)))
	logs := plog.NewLogs()
	meta := MetaToKeyVal(payload.Meta)
	resourceAttrs := map[string]any{
		string(semconv.ServiceNameKey):           payload.Meta.App.Name,
		string(semconv.ServiceVersionKey):        payload.Meta.App.Version,
		string(semconv.DeploymentEnvironmentKey): payload.Meta.App.Environment,
	}
	if payload.Meta.App.Namespace != "" {
		resourceAttrs[string(semconv.ServiceNamespaceKey)] = payload.Meta.App.Namespace
	}
	if payload.Meta.App.BundleID != "" {
		resourceAttrs["app_bundle_id"] = payload.Meta.App.BundleID
	}
	rls := logs.ResourceLogs().AppendEmpty()
	if err := rls.Resource().Attributes().FromRaw(resourceAttrs); err != nil {
		return nil, err
	}
	sl := rls.ScopeLogs().AppendEmpty()
	attrs := pcommon.NewMap()
	for _, i := range kvList {
		MergeKeyVal(i.kv, meta)
		line, err := logfmt.MarshalKeyvals(KeyValToInterfaceSlice(i.kv)...)
		if err != nil {
			return nil, err
		}
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.Body().SetStr(string(line))
		attrs.CopyTo(logRecord.Attributes())
		logRecord.Attributes().PutStr("kind", string(i.kind))
		if (i.kind == faroTypes.KindException) && (i.hash != 0) {
			logRecord.Attributes().PutStr("hash", strconv.FormatUint(i.hash, 10))
		}
	}
	return &logs, nil
}
