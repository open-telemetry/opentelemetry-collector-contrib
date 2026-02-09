// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/service/hostcapabilities"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var (
	_ exporter.Logs    = (*exporterCreator)(nil)
	_ exporter.Metrics = (*exporterCreator)(nil)
	_ exporter.Traces  = (*exporterCreator)(nil)
	_ consumer.Logs    = (*exporterCreator)(nil)
	_ consumer.Metrics = (*exporterCreator)(nil)
	_ consumer.Traces  = (*exporterCreator)(nil)
)

// exporterCreator implements the exporter that dynamically creates sub-exporters at runtime.
type exporterCreator struct {
	params           exporter.Settings
	cfg              *Config
	observerHandler  *observerHandler
	observables      []observer.Observable
	router           *telemetryRouter
	telemetry        *metadata.TelemetryBuilder
	defaultExporters map[component.ID]component.Component
}

// host is an interface that the component.Host passed to exportercreator's Start function must implement
type host interface {
	component.Host
	hostcapabilities.ComponentFactory
}

func newExporterCreator(params exporter.Settings, cfg *Config) (*exporterCreator, error) {
	telemetry, err := metadata.NewTelemetryBuilder(params.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	router := newTelemetryRouter(cfg.Routing.Rules, telemetry)
	router.setLogger(params.Logger)
	ec := &exporterCreator{
		params:    params,
		cfg:       cfg,
		router:    router,
		telemetry: telemetry,
	}
	return ec, nil
}

// Start exporter_creator.
func (ec *exporterCreator) Start(ctx context.Context, h component.Host) error {
	ecHost, ok := h.(host)
	if !ok {
		return errors.New("the exporter_creator is not compatible with the provided component.host")
	}

	// Initialize the gauge metric to 0
	if ec.telemetry != nil {
		ec.telemetry.ExporterCreatorExportersCount.Record(ctx, 0)
	}

	// Get default exporters from host
	// Note: Default exporters should be static exporters configured in the service pipelines
	// For now, we'll store the IDs and look them up when needed
	ec.defaultExporters = make(map[component.ID]component.Component)
	// TODO: Implement proper default exporter lookup from host
	// This requires access to the service's exporter registry which may not be available
	// through the standard component.Host interface

	// Create observer handler
	ec.observerHandler = &observerHandler{
		config:              ec.cfg,
		params:              ec.params,
		exportersByEndpoint: make(map[observer.EndpointID]component.Component),
		router:              ec.router,
		runner:              newExporterRunner(ec.params, ecHost),
	}

	observers := map[component.ID]observer.Observable{}

	// Match all configured observables to the extensions that are running.
	for _, watchObserver := range ec.cfg.WatchObservers {
		for cid, ext := range ecHost.GetExtensions() {
			if cid != watchObserver {
				continue
			}

			obs, ok := ext.(observer.Observable)
			if !ok {
				return fmt.Errorf("extension %q in watch_observers is not an observer", watchObserver.String())
			}
			observers[watchObserver] = obs
		}
	}

	// Make sure all observables are present before starting any.
	for _, watchObserver := range ec.cfg.WatchObservers {
		if observers[watchObserver] == nil {
			return fmt.Errorf("failed to find observer %q in the extensions list", watchObserver.String())
		}
	}

	if len(observers) == 0 {
		ec.params.Logger.Warn("no observers were configured and no subexporters will be started. exporter_creator will be disabled")
	}

	// Start all configured watchers.
	for _, observable := range observers {
		ec.observables = append(ec.observables, observable)
		observable.ListAndWatch(ec.observerHandler)
	}

	return nil
}

// Shutdown stops the exporter_creator and all its exporters started at runtime.
func (ec *exporterCreator) Shutdown(ctx context.Context) error {
	var errs []error

	// Unsubscribe from all observers
	for _, observable := range ec.observables {
		observable.Unsubscribe(ec.observerHandler)
	}

	// Shutdown observer handler (which shuts down all sub-exporters)
	if ec.observerHandler != nil {
		if err := ec.observerHandler.shutdown(); err != nil {
			errs = append(errs, err)
		}
	}

	if ec.telemetry != nil {
		ec.telemetry.Shutdown()
	}

	if len(errs) > 0 {
		return multierr.Combine(errs...)
	}
	return nil
}

// Capabilities implements consumer.Logs, consumer.Metrics, consumer.Traces.
func (ec *exporterCreator) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs routes logs to the appropriate sub-exporter based on resource attributes.
func (ec *exporterCreator) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if ec.router == nil {
		return nil
	}

	var errs []error
	exportersByComponent := make(map[component.Component]plog.Logs)
	unmatchedLogs := plog.NewLogs()

	// Group logs by matching exporters
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		resourceAttrs := rl.Resource().Attributes()

		// Find matching exporters
		matchedExporters := ec.router.Route(resourceAttrs)

		if len(matchedExporters) > 0 {
			// Route to matched exporters
			for _, exp := range matchedExporters {
				if _, ok := exp.(exporter.Logs); ok {
					if _, exists := exportersByComponent[exp]; !exists {
						exportersByComponent[exp] = plog.NewLogs()
					}
					rl.CopyTo(exportersByComponent[exp].ResourceLogs().AppendEmpty())
				}
			}
		} else {
			// No match, add to unmatched
			rl.CopyTo(unmatchedLogs.ResourceLogs().AppendEmpty())
		}
	}

	// Send to matched exporters
	for exp, logs := range exportersByComponent {
		if logsExp, ok := exp.(exporter.Logs); ok {
			if err := logsExp.ConsumeLogs(ctx, logs); err != nil {
				errs = append(errs, fmt.Errorf("failed to export logs to exporter: %w", err))
			}
		}
	}

	// Route unmatched logs to default exporters
	if unmatchedLogs.ResourceLogs().Len() > 0 && len(ec.defaultExporters) > 0 {
		for _, defaultExp := range ec.defaultExporters {
			if logsExp, ok := defaultExp.(exporter.Logs); ok {
				if err := logsExp.ConsumeLogs(ctx, unmatchedLogs); err != nil {
					errs = append(errs, fmt.Errorf("failed to export logs to default exporter: %w", err))
				}
			}
		}
	}

	if len(errs) > 0 {
		return multierr.Combine(errs...)
	}
	return nil
}

// ConsumeMetrics routes metrics to the appropriate sub-exporter based on resource attributes.
func (ec *exporterCreator) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if ec.router == nil {
		return nil
	}

	var errs []error
	exportersByComponent := make(map[component.Component]pmetric.Metrics)
	unmatchedMetrics := pmetric.NewMetrics()

	// Group metrics by matching exporters
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resourceAttrs := rm.Resource().Attributes()

		// Find matching exporters
		matchedExporters := ec.router.Route(resourceAttrs)

		if len(matchedExporters) > 0 {
			// Route to matched exporters
			for _, exp := range matchedExporters {
				if _, ok := exp.(exporter.Metrics); ok {
					if _, exists := exportersByComponent[exp]; !exists {
						exportersByComponent[exp] = pmetric.NewMetrics()
					}
					rm.CopyTo(exportersByComponent[exp].ResourceMetrics().AppendEmpty())
				}
			}
		} else {
			// No match, add to unmatched
			// Log debug info about why routing failed
			if ce := ec.params.Logger.Check(zap.DebugLevel, "metrics did not match any routing rules"); ce != nil {
				// Convert resource attrs to map for logging
				resourceAttrsMap := make(map[string]string)
				resourceAttrs.Range(func(k string, v pcommon.Value) bool {
					resourceAttrsMap[k] = v.AsString()
					return true
				})
				ce.Write(
					zap.Any("resource_attributes", resourceAttrsMap),
					zap.Int("available_exporters", ec.router.Count()),
				)
			}
			rm.CopyTo(unmatchedMetrics.ResourceMetrics().AppendEmpty())
		}
	}

	// Send to matched exporters
	for exp, metrics := range exportersByComponent {
		if metricsExp, ok := exp.(exporter.Metrics); ok {
			if err := metricsExp.ConsumeMetrics(ctx, metrics); err != nil {
				errs = append(errs, fmt.Errorf("failed to export metrics to exporter: %w", err))
			}
		}
	}

	// Route unmatched metrics to default exporters
	nonRoutableCount := int64(0)
	if unmatchedMetrics.ResourceMetrics().Len() > 0 {
		if len(ec.defaultExporters) > 0 {
			hasMetricsExporter := false
			exportSucceeded := false
			for _, defaultExp := range ec.defaultExporters {
				if metricsExp, ok := defaultExp.(exporter.Metrics); ok {
					hasMetricsExporter = true
					if err := metricsExp.ConsumeMetrics(ctx, unmatchedMetrics); err != nil {
						errs = append(errs, fmt.Errorf("failed to export metrics to default exporter: %w", err))
					} else {
						exportSucceeded = true
					}
				}
			}
			// If no default exporter supports metrics, or export failed, count all unmatched as non-routable
			if !hasMetricsExporter || !exportSucceeded {
				nonRoutableCount = countMetricPoints(unmatchedMetrics)
			}
		} else {
			// No default exporters configured, count all unmatched as non-routable
			nonRoutableCount = countMetricPoints(unmatchedMetrics)
		}
	}

	// Record non-routable metric points
	if nonRoutableCount > 0 && ec.telemetry != nil {
		ec.telemetry.ExporterCreatorNonroutableMetricPointsTotal.Add(ctx, nonRoutableCount)
	}

	if len(errs) > 0 {
		return multierr.Combine(errs...)
	}
	return nil
}

// countMetricPoints counts the total number of data points in a pmetric.Metrics.
func countMetricPoints(md pmetric.Metrics) int64 {
	var count int64
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				//exhaustive:enforce
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					count += int64(metric.Gauge().DataPoints().Len())
				case pmetric.MetricTypeSum:
					count += int64(metric.Sum().DataPoints().Len())
				case pmetric.MetricTypeHistogram:
					count += int64(metric.Histogram().DataPoints().Len())
				case pmetric.MetricTypeExponentialHistogram:
					count += int64(metric.ExponentialHistogram().DataPoints().Len())
				case pmetric.MetricTypeSummary:
					count += int64(metric.Summary().DataPoints().Len())
				case pmetric.MetricTypeEmpty:
					// Empty metrics have no data points
				}
			}
		}
	}
	return count
}

// ConsumeTraces routes traces to the appropriate sub-exporter based on resource attributes.
func (ec *exporterCreator) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if ec.router == nil {
		return nil
	}

	var errs []error
	exportersByComponent := make(map[component.Component]ptrace.Traces)
	unmatchedTraces := ptrace.NewTraces()

	// Group traces by matching exporters
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		resourceAttrs := rs.Resource().Attributes()

		// Find matching exporters
		matchedExporters := ec.router.Route(resourceAttrs)

		if len(matchedExporters) > 0 {
			// Route to matched exporters
			for _, exp := range matchedExporters {
				if _, ok := exp.(exporter.Traces); ok {
					if _, exists := exportersByComponent[exp]; !exists {
						exportersByComponent[exp] = ptrace.NewTraces()
					}
					rs.CopyTo(exportersByComponent[exp].ResourceSpans().AppendEmpty())
				}
			}
		} else {
			// No match, add to unmatched
			rs.CopyTo(unmatchedTraces.ResourceSpans().AppendEmpty())
		}
	}

	// Send to matched exporters
	for exp, traces := range exportersByComponent {
		if tracesExp, ok := exp.(exporter.Traces); ok {
			if err := tracesExp.ConsumeTraces(ctx, traces); err != nil {
				errs = append(errs, fmt.Errorf("failed to export traces to exporter: %w", err))
			}
		}
	}

	// Route unmatched traces to default exporters
	if unmatchedTraces.ResourceSpans().Len() > 0 && len(ec.defaultExporters) > 0 {
		for _, defaultExp := range ec.defaultExporters {
			if tracesExp, ok := defaultExp.(exporter.Traces); ok {
				if err := tracesExp.ConsumeTraces(ctx, unmatchedTraces); err != nil {
					errs = append(errs, fmt.Errorf("failed to export traces to default exporter: %w", err))
				}
			}
		}
	}

	if len(errs) > 0 {
		return multierr.Combine(errs...)
	}
	return nil
}
