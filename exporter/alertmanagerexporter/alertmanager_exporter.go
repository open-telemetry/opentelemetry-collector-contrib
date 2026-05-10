// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertmanagerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alertmanagerexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type alertmanagerExporter struct {
	config            *Config
	client            *http.Client
	tracesMarshaler   ptrace.Marshaler
	settings          component.TelemetrySettings
	endpoint          string
	generatorURL      string
	defaultSeverity   string
	severityAttribute string
	apiVersion        string
}

type alertmanagerEvent struct {
	spanEvent ptrace.SpanEvent
	traceID   string
	spanID    string
	severity  string
}

type alertmanagerLogRecord struct {
	logRecord plog.LogRecord
	traceID   string
	spanID    string
	severity  string
	name      string
}

func (s *alertmanagerExporter) convertEventSliceToArray(eventSlice ptrace.SpanEventSlice, traceID pcommon.TraceID, spanID pcommon.SpanID) []*alertmanagerEvent {
	if eventSlice.Len() > 0 {
		events := make([]*alertmanagerEvent, eventSlice.Len())

		for i := 0; i < eventSlice.Len(); i++ {
			var severity string
			severityAttrValue, ok := eventSlice.At(i).Attributes().Get(s.severityAttribute)
			if ok {
				severity = severityAttrValue.AsString()
			} else {
				severity = s.defaultSeverity
			}
			event := alertmanagerEvent{
				spanEvent: eventSlice.At(i),
				traceID:   traceID.String(),
				spanID:    spanID.String(),
				severity:  severity,
			}

			events[i] = &event
		}
		return events
	}
	return nil
}

func logRecordName(record plog.LogRecord) string {
	if nameAttr, ok := record.Attributes().Get("event.name"); ok {
		if name := nameAttr.AsString(); name != "" {
			return name
		}
	}
	return "log_record"
}

func (s *alertmanagerExporter) extractEvents(td ptrace.Traces) []*alertmanagerEvent {
	// Stitch parent trace ID and span ID
	rss := td.ResourceSpans()
	var events []*alertmanagerEvent
	if rss.Len() == 0 {
		return nil
	}

	for i := 0; i < rss.Len(); i++ {
		resource := rss.At(i).Resource()
		ilss := rss.At(i).ScopeSpans()

		if resource.Attributes().Len() == 0 && ilss.Len() == 0 {
			return nil
		}

		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				traceID := spans.At(k).TraceID()
				spanID := spans.At(k).SpanID()
				events = append(events, s.convertEventSliceToArray(spans.At(k).Events(), traceID, spanID)...)
			}
		}
	}
	return events
}

func (s *alertmanagerExporter) extractLogs(ld plog.Logs) []*alertmanagerLogRecord {
	rls := ld.ResourceLogs()
	if rls.Len() == 0 {
		return nil
	}

	var records []*alertmanagerLogRecord
	for i := 0; i < rls.Len(); i++ {
		sls := rls.At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			logs := sls.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				record := logs.At(k)
				severity := s.defaultSeverity
				if s.severityAttribute != "" {
					if severityAttrValue, ok := record.Attributes().Get(s.severityAttribute); ok {
						if value := severityAttrValue.AsString(); value != "" {
							severity = value
						}
					}
				}
				if severity == s.defaultSeverity {
					if severityText := record.SeverityText(); severityText != "" {
						severity = severityText
					}
				}
				records = append(records, &alertmanagerLogRecord{
					logRecord: record,
					traceID:   record.TraceID().String(),
					spanID:    record.SpanID().String(),
					severity:  severity,
					name:      logRecordName(record),
				})
			}
		}
	}
	return records
}

func createAnnotations(event *alertmanagerEvent) model.LabelSet {
	labelMap := make(model.LabelSet, event.spanEvent.Attributes().Len()+2)
	for key, attr := range event.spanEvent.Attributes().All() {
		labelMap[model.LabelName(key)] = model.LabelValue(attr.AsString())
	}
	labelMap["TraceID"] = model.LabelValue(event.traceID)
	labelMap["SpanID"] = model.LabelValue(event.spanID)
	return labelMap
}

func createLogAnnotations(record *alertmanagerLogRecord) model.LabelSet {
	labelMap := make(model.LabelSet, record.logRecord.Attributes().Len()+3)
	for key, attr := range record.logRecord.Attributes().All() {
		labelMap[model.LabelName(key)] = model.LabelValue(attr.AsString())
	}
	if body := record.logRecord.Body().AsString(); body != "" {
		labelMap["body"] = model.LabelValue(body)
	}
	labelMap["TraceID"] = model.LabelValue(record.traceID)
	labelMap["SpanID"] = model.LabelValue(record.spanID)
	return labelMap
}

func (s *alertmanagerExporter) createLabels(event *alertmanagerEvent) model.LabelSet {
	labelMap := model.LabelSet{}
	for key, attr := range event.spanEvent.Attributes().All() {
		if slices.Contains(s.config.EventLabels, key) {
			labelMap[model.LabelName(key)] = model.LabelValue(attr.AsString())
		}
	}
	labelMap["severity"] = model.LabelValue(event.severity)
	labelMap["event_name"] = model.LabelValue(event.spanEvent.Name())
	return labelMap
}

func (s *alertmanagerExporter) createLogLabels(record *alertmanagerLogRecord) model.LabelSet {
	labelMap := model.LabelSet{}
	for key, attr := range record.logRecord.Attributes().All() {
		if slices.Contains(s.config.EventLabels, key) {
			labelMap[model.LabelName(key)] = model.LabelValue(attr.AsString())
		}
	}
	labelMap["severity"] = model.LabelValue(record.severity)
	labelMap["event_name"] = model.LabelValue(record.name)
	return labelMap
}

func (s *alertmanagerExporter) convertEventsToAlertPayload(events []*alertmanagerEvent) []model.Alert {
	payload := make([]model.Alert, len(events))

	for i, event := range events {
		annotations := createAnnotations(event)
		labels := s.createLabels(event)

		alert := model.Alert{
			StartsAt:     time.Now(),
			Labels:       labels,
			Annotations:  annotations,
			GeneratorURL: s.generatorURL,
		}

		payload[i] = alert
	}
	return payload
}

func (s *alertmanagerExporter) convertLogRecordsToAlertPayload(records []*alertmanagerLogRecord) []model.Alert {
	payload := make([]model.Alert, len(records))

	for i, record := range records {
		annotations := createLogAnnotations(record)
		labels := s.createLogLabels(record)

		alert := model.Alert{
			StartsAt:     time.Now(),
			Labels:       labels,
			Annotations:  annotations,
			GeneratorURL: s.generatorURL,
		}

		payload[i] = alert
	}
	return payload
}

func (s *alertmanagerExporter) postAlert(ctx context.Context, payload []model.Alert) error {
	msg, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling alert to JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewBuffer(msg))
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending HTTP request: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.settings.Logger.Warn("failed to close response body", zap.Error(closeErr))
		}
	}()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		s.settings.Logger.Debug("post request to Alertmanager failed", zap.Error(err))
		return fmt.Errorf("request POST %s failed - %q", req.URL.String(), resp.Status)
	}
	return nil
}

func (s *alertmanagerExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	events := s.extractEvents(td)

	if len(events) == 0 {
		return nil
	}

	alert := s.convertEventsToAlertPayload(events)
	err := s.postAlert(ctx, alert)
	if err != nil {
		return err
	}

	return nil
}

func (s *alertmanagerExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	records := s.extractLogs(ld)

	if len(records) == 0 {
		return nil
	}

	alert := s.convertLogRecordsToAlertPayload(records)
	if err := s.postAlert(ctx, alert); err != nil {
		return err
	}

	return nil
}

func (s *alertmanagerExporter) start(ctx context.Context, host component.Host) error {
	client, err := s.config.ToClient(ctx, host.GetExtensions(), s.settings)
	if err != nil {
		return fmt.Errorf("failed to create HTTP Client: %w", err)
	}
	s.client = client
	return nil
}

func (s *alertmanagerExporter) shutdown(context.Context) error {
	if s.client != nil {
		s.client.CloseIdleConnections()
	}
	return nil
}

func newAlertManagerExporter(cfg *Config, set component.TelemetrySettings) *alertmanagerExporter {
	return &alertmanagerExporter{
		config:            cfg,
		settings:          set,
		tracesMarshaler:   &ptrace.JSONMarshaler{},
		endpoint:          fmt.Sprintf("%s/api/%s/alerts", cfg.Endpoint, cfg.APIVersion),
		generatorURL:      cfg.GeneratorURL,
		defaultSeverity:   cfg.DefaultSeverity,
		severityAttribute: cfg.SeverityAttribute,
		apiVersion:        cfg.APIVersion,
	}
}

func newTracesExporter(ctx context.Context, cfg component.Config, set exporter.Settings) (exporter.Traces, error) {
	config := cfg.(*Config)

	s := newAlertManagerExporter(config, set.TelemetrySettings)

	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		s.pushTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithStart(s.start),
		exporterhelper.WithTimeout(config.TimeoutSettings),
		exporterhelper.WithRetry(config.BackoffConfig),
		exporterhelper.WithQueue(config.QueueSettings),
		exporterhelper.WithShutdown(s.shutdown),
	)
}

func newLogsExporter(ctx context.Context, cfg component.Config, set exporter.Settings) (exporter.Logs, error) {
	config := cfg.(*Config)

	s := newAlertManagerExporter(config, set.TelemetrySettings)

	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		s.pushLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithStart(s.start),
		exporterhelper.WithTimeout(config.TimeoutSettings),
		exporterhelper.WithRetry(config.BackoffConfig),
		exporterhelper.WithQueue(config.QueueSettings),
		exporterhelper.WithShutdown(s.shutdown),
	)
}
