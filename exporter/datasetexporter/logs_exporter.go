// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter"

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/scalyr/dataset-go/pkg/api/add_events"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func createLogsExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Logs, error) {
	cfg := castConfig(config)
	e, err := newDatasetExporter("logs", cfg, set.Logger)
	if err != nil {
		return nil, fmt.Errorf("cannot get DataSetExpoter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		config,
		e.consumeLogs,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithShutdown(e.shutdown),
	)
}

func buildBody(attrs map[string]interface{}, value pcommon.Value) string {
	message := value.AsString()
	attrs["body.type"] = value.Type().String()
	switch value.Type() {
	case pcommon.ValueTypeEmpty:
		attrs["body.empty"] = value.AsString()
	case pcommon.ValueTypeStr:
		attrs["body.str"] = value.Str()
	case pcommon.ValueTypeBool:
		attrs["body.bool"] = value.Bool()
	case pcommon.ValueTypeDouble:
		attrs["body.double"] = value.Double()
	case pcommon.ValueTypeInt:
		attrs["body.int"] = value.Int()
	case pcommon.ValueTypeMap:
		updateWithPrefixedValues(attrs, "body.map.", ".", value.Map().AsRaw(), 0)
	case pcommon.ValueTypeBytes:
		attrs["body.bytes"] = value.AsString()
	case pcommon.ValueTypeSlice:
		attrs["body.slice"] = value.AsRaw()
	default:
		attrs["body.unknown"] = value.AsString()
	}

	return message
}

func buildEventFromLog(
	log plog.LogRecord,
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	settings LogsSettings,
) *add_events.EventBundle {
	attrs := make(map[string]interface{})
	event := add_events.Event{}

	if sevNum := log.SeverityNumber(); sevNum > 0 {
		attrs["severity.number"] = sevNum
		event.Sev = int(sevNum)
	}

	if timestamp := log.Timestamp().AsTime(); !timestamp.Equal(time.Unix(0, 0)) {
		attrs["timestamp"] = timestamp.String()
		event.Ts = strconv.FormatInt(timestamp.UnixNano(), 10)
	}

	if body := log.Body().AsString(); body != "" {
		attrs["message"] = fmt.Sprintf(
			"OtelExporter - Log - %s",
			buildBody(attrs, log.Body()),
		)
	}
	if dropped := log.DroppedAttributesCount(); dropped > 0 {
		attrs["dropped_attributes_count"] = dropped
	}
	if observed := log.ObservedTimestamp().AsTime(); !observed.Equal(time.Unix(0, 0)) {
		attrs["observed.timestamp"] = observed.String()
	}
	if sevText := log.SeverityText(); sevText != "" {
		attrs["severity.text"] = sevText
	}
	if span := log.SpanID().String(); span != "" {
		attrs["span_id"] = span
	}

	if trace := log.TraceID().String(); trace != "" {
		attrs["trace_id"] = trace
	}

	updateWithPrefixedValues(attrs, "attributes.", ".", log.Attributes().AsRaw(), 0)
	attrs["flags"] = log.Flags()
	attrs["flag.is_sampled"] = log.Flags().IsSampled()

	if settings.ExportResourceInfo {
		updateWithPrefixedValues(attrs, "resource.attributes.", ".", resource.Attributes().AsRaw(), 0)
	}
	attrs["scope.name"] = scope.Name()
	updateWithPrefixedValues(attrs, "scope.attributes.", ".", scope.Attributes().AsRaw(), 0)

	event.Attrs = attrs
	event.Log = "LL"
	event.Thread = "TL"
	return &add_events.EventBundle{
		Event:  &event,
		Thread: &add_events.Thread{Id: "TL", Name: "logs"},
		Log:    &add_events.Log{Id: "LL", Attrs: map[string]interface{}{}},
	}
}

func (e *DatasetExporter) consumeLogs(_ context.Context, ld plog.Logs) error {
	var events []*add_events.EventBundle

	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		resource := resourceLogs.At(i).Resource()
		scopeLogs := resourceLogs.At(i).ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			scope := scopeLogs.At(j).Scope()
			logRecords := scopeLogs.At(j).LogRecords()
			for k := 0; k < logRecords.Len(); k++ {
				logRecord := logRecords.At(k)
				events = append(events, buildEventFromLog(logRecord, resource, scope, e.exporterCfg.logsSettings))
			}
		}
	}

	return sendBatch(events, e.client)
}
