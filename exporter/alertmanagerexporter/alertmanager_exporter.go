// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertmanagerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alertmanagerexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	// "go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type alertmanagerExporter struct {
	config            *Config
	client            *http.Client
	tracesMarshaler   ptrace.Marshaler
	settings          component.TelemetrySettings
	endpoint          string
	generatorUrl      string
	defaultSeverity   string
	severityAttribute string
}

type alertmanagerEvent struct {
	spanEvent ptrace.SpanEvent
	traceID   string
	spanID    string
	severity  string
}

func (s *alertmanagerExporter) convertEventSliceToArray(eventslice ptrace.SpanEventSlice, traceID pcommon.TraceID, spanID pcommon.SpanID) []*alertmanagerEvent {
	if eventslice.Len() > 0 {
		events := make([]*alertmanagerEvent, eventslice.Len())

		for i := 0; i < eventslice.Len(); i++ {
			var severity string
			// severity := pcommon.NewValueStr(s.defaultSeverity)
			severityAttrValue, ok := eventslice.At(i).Attributes().Get(s.severityAttribute)
			if ok {
				severity = severityAttrValue.AsString()
			} else {
				severity = s.defaultSeverity
			}
			event := alertmanagerEvent{
				spanEvent: eventslice.At(i),
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

func (s *alertmanagerExporter) extractEvents(td ptrace.Traces) []*alertmanagerEvent {

	//Stitch parent trace ID and span ID
	rss := td.ResourceSpans()
	var events []*alertmanagerEvent = nil
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
				traceID := pcommon.TraceID(spans.At(k).TraceID())
				spanID := pcommon.SpanID(spans.At(k).SpanID())
				events = append(events, s.convertEventSliceToArray(spans.At(k).Events(), traceID, spanID)...)
			}
		}
	}
	return events
}

func createAnnotations(event *alertmanagerEvent) model.LabelSet {
	LabelMap := make(model.LabelSet, event.spanEvent.Attributes().Len()+1)
	event.spanEvent.Attributes().Range(func(key string, attr pcommon.Value) bool {
		LabelMap[model.LabelName(key)] = model.LabelValue(attr.AsString())
		return true
	})
	LabelMap["TraceID"] = model.LabelValue(event.traceID)
	LabelMap["SpanID"] = model.LabelValue(event.spanID)
	return LabelMap
}

func (s *alertmanagerExporter) convertEventstoAlertPayload(events []*alertmanagerEvent) []model.Alert {

	var payload []model.Alert
	for _, event := range events {
		annotations := createAnnotations(event)

		alert := model.Alert{
			StartsAt:     time.Now(),
			Labels:       model.LabelSet{"severity": model.LabelValue(event.severity), "event_name": model.LabelValue(event.spanEvent.Name())},
			Annotations:  annotations,
			GeneratorURL: s.generatorUrl,
		}

		payload = append(payload, alert)
	}
	return payload
}

func (s *alertmanagerExporter) postAlert(ctx context.Context, payload []model.Alert) error {
	msg, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", s.endpoint, bytes.NewBuffer(msg))
	if err != nil {
		s.settings.Logger.Debug("error creating HTTP request", zap.Error(err))
		return fmt.Errorf("error creating HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-protobuf")

	resp, err := s.client.Do(req)
	if err != nil {
		s.settings.Logger.Debug("error sending HTTP request", zap.Error(err))
		return fmt.Errorf("error sending HTTP request: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.settings.Logger.Warn("Failed to close response body", zap.Error(closeErr))
		}
	}()

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		s.settings.Logger.Debug("failed to read response body", zap.Error(err))
		return fmt.Errorf("failed to read response body %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		s.settings.Logger.Debug("post request to Alertmanager failed", zap.Error(err))
		return fmt.Errorf("request POST %s failed - %q", req.URL.String(), resp.Status)
	}
	return nil
}

func (s *alertmanagerExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {

	s.settings.Logger.Debug("TracesExporter", zap.Int("#spans", td.SpanCount()))

	events := s.extractEvents(td)

	if len(events) == 0 {
		return nil
	}

	alert := s.convertEventstoAlertPayload(events)
	err := s.postAlert(ctx, alert)

	if err != nil {
		return err
	}

	return nil
}

func (s *alertmanagerExporter) start(_ context.Context, host component.Host) error {

	client, err := s.config.HTTPClientSettings.ToClient(host, s.settings)
	if err != nil {
		s.settings.Logger.Error("failed to create HTTP Client", zap.Error(err))
		return fmt.Errorf("failed to create HTTP Client: %w", err)
	}
	s.client = client
	return nil
}

func (s *alertmanagerExporter) shutdown(context.Context) error {
	return nil
}

func newAlertManagerExporter(cfg *Config, set component.TelemetrySettings) *alertmanagerExporter {

	url := cfg.GeneratorURL

	if len(url) == 0 {
		url = "http://example.com/alert"
	}

	severity := cfg.DefaultSeverity

	if len(severity) == 0 {
		severity = "info"
	}

	return &alertmanagerExporter{
		config:            cfg,
		settings:          set,
		tracesMarshaler:   &ptrace.JSONMarshaler{},
		endpoint:          fmt.Sprintf("%s/api/v1/alerts", cfg.HTTPClientSettings.Endpoint),
		generatorUrl:      url,
		defaultSeverity:   severity,
		severityAttribute: cfg.SeverityAttribute,
	}
}

func newTracesExporter(ctx context.Context, cfg component.Config, set exporter.CreateSettings) (exporter.Traces, error) {
	config := cfg.(*Config)

	if len(config.HTTPClientSettings.Endpoint) == 0 {
		return nil, errors.New("endpoint is not set")
	}

	s := newAlertManagerExporter(config, set.TelemetrySettings)

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		s.pushTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		// Disable Timeout/RetryOnFailure and SendingQueue
		exporterhelper.WithStart(s.start),
		exporterhelper.WithTimeout(config.TimeoutSettings),
		exporterhelper.WithRetry(config.RetrySettings),
		exporterhelper.WithQueue(config.QueueSettings),
		exporterhelper.WithShutdown(s.shutdown),
	)
}
