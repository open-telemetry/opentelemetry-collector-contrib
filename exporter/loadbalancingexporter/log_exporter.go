// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/metric"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

const (
	pseudoAttrLogSeverity = "log.severity"
	pseudoAttrLogBody     = "log.body"
)

var _ exporter.Logs = (*logExporterImp)(nil)

type logExporterImp struct {
	loadBalancer *loadBalancer
	routingKey   routingKey
	routingAttrs []string

	logger     *zap.Logger
	started    bool
	shutdownWg sync.WaitGroup
	telemetry  *metadata.TelemetryBuilder
}

// Create new logs exporter
func newLogsExporter(params exporter.Settings, cfg component.Config) (*logExporterImp, error) {
	telemetry, err := metadata.NewTelemetryBuilder(params.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	exporterFactory := otlpexporter.NewFactory()
	cfFunc := func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := buildExporterConfig(cfg.(*Config), endpoint)
		oParams := buildExporterSettings(exporterFactory.Type(), params, endpoint)

		return exporterFactory.CreateLogs(ctx, oParams, &oCfg)
	}

	lb, err := newLoadBalancer(params.Logger, cfg, cfFunc, telemetry)
	if err != nil {
		return nil, err
	}

	logExporter := logExporterImp{
		loadBalancer: lb,
		routingKey:   svcRouting,
		telemetry:    telemetry,
		logger:       params.Logger,
	}

	switch cfg.(*Config).RoutingKey {
	case svcRoutingStr, "":
		logExporter.routingKey = svcRouting
	case resourceRoutingStr:
		logExporter.routingKey = resourceRouting
	case attrRoutingStr:
		logExporter.routingKey = attrRouting
		logExporter.routingAttrs = cfg.(*Config).RoutingAttributes
	default:
		return nil, fmt.Errorf("unsupported routing_key: %q", cfg.(*Config).RoutingKey)
	}

	return &logExporter, nil
}

func (*logExporterImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *logExporterImp) Start(ctx context.Context, host component.Host) error {
	e.started = true
	return e.loadBalancer.Start(ctx, host)
}

func (e *logExporterImp) Shutdown(ctx context.Context) error {
	if !e.started {
		return nil
	}
	err := e.loadBalancer.Shutdown(ctx)
	e.started = false
	e.shutdownWg.Wait()
	return err
}

func (e *logExporterImp) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var batches map[string]plog.Logs

	switch e.routingKey {
	case svcRouting:
		var errs []error
		batches, errs = splitLogsByServiceName(ld)
		if len(errs) > 0 {
			for _, ee := range errs {
				e.logger.Error("failed to export log", zap.Error(ee))
			}
			if len(batches) == 0 {
				return consumererror.NewPermanent(errors.Join(errs...))
			}
		}
	case resourceRouting:
		batches = splitLogsByResourceID(ld)
	case attrRouting:
		batches = splitLogsByAttributes(ld, e.routingAttrs)
	}

	logsByExporter := map[*wrappedExporter]plog.Logs{}
	exporterEndpoints := map[*wrappedExporter]string{}

	for routingID, lds := range batches {
		exp, endpoint, err := e.loadBalancer.exporterAndEndpoint([]byte(routingID))
		if err != nil {
			return err
		}

		_, ok := logsByExporter[exp]
		if !ok {
			exp.consumeWG.Add(1)
			logsByExporter[exp] = plog.NewLogs()
			exporterEndpoints[exp] = endpoint
		}

		mergeLogs(logsByExporter[exp], lds)
	}

	var errs error
	for exp, lds := range logsByExporter {
		start := time.Now()
		err := exp.ConsumeLogs(ctx, lds)
		duration := time.Since(start)

		exp.consumeWG.Done()
		errs = multierr.Append(errs, err)
		e.telemetry.LoadbalancerBackendLatency.Record(ctx, duration.Milliseconds(), metric.WithAttributeSet(exp.endpointAttr))
		if err == nil {
			e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(exp.successAttr))
		} else {
			e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(exp.failureAttr))
			e.logger.Debug("failed to export logs", zap.Error(err))
		}
	}

	return errs
}

func splitLogsByServiceName(ld plog.Logs) (map[string]plog.Logs, []error) {
	results := map[string]plog.Logs{}
	var errs []error

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rm := ld.ResourceLogs().At(i)

		svc, ok := rm.Resource().Attributes().Get(string(conventions.ServiceNameKey))
		if !ok {
			errs = append(errs, fmt.Errorf("unable to get service name from resource log with attributes: %v", rm.Resource().Attributes().AsRaw()))
			continue
		}

		newLD := plog.NewLogs()
		rmClone := newLD.ResourceLogs().AppendEmpty()
		rm.CopyTo(rmClone)

		key := svc.Str()
		existing, ok := results[key]
		if ok {
			mergeLogs(existing, newLD)
		} else {
			results[key] = newLD
		}
	}

	return results, errs
}

func splitLogsByResourceID(ld plog.Logs) map[string]plog.Logs {
	results := map[string]plog.Logs{}

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rm := ld.ResourceLogs().At(i)

		newLD := plog.NewLogs()
		rmClone := newLD.ResourceLogs().AppendEmpty()
		rm.CopyTo(rmClone)

		key := identity.OfResource(rm.Resource()).String()
		existing, ok := results[key]
		if ok {
			mergeLogs(existing, newLD)
		} else {
			results[key] = newLD
		}
	}

	return results
}

func splitLogsByAttributes(ld plog.Logs, attrs []string) map[string]plog.Logs {
	results := map[string]plog.Logs{}

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rm := ld.ResourceLogs().At(i)

		var rKey strings.Builder

		for _, a := range attrs {
			// resource attributes
			rAttr, ok := rm.Resource().Attributes().Get(a)
			if ok {
				rKey.WriteString(rAttr.Str())
				continue
			}

			// scope attributes
			if rm.ScopeLogs().Len() > 0 {
				sAttr, ok := rm.ScopeLogs().At(0).Scope().Attributes().Get(a)
				if ok {
					rKey.WriteString(sAttr.Str())
					continue
				}

				// log record attributes and pseudo attributes
				if rm.ScopeLogs().At(0).LogRecords().Len() > 0 {
					lr := rm.ScopeLogs().At(0).LogRecords().At(0)

					if a == pseudoAttrLogSeverity {
						rKey.WriteString(lr.SeverityText())
						continue
					}

					if a == pseudoAttrLogBody {
						rKey.WriteString(lr.Body().AsString())
						continue
					}

					lAttr, ok := lr.Attributes().Get(a)
					if ok {
						rKey.WriteString(lAttr.Str())
						continue
					}
				}
			}
		}

		newLD := plog.NewLogs()
		rmClone := newLD.ResourceLogs().AppendEmpty()
		rm.CopyTo(rmClone)

		key := rKey.String()
		existing, ok := results[key]
		if ok {
			mergeLogs(existing, newLD)
		} else {
			results[key] = newLD
		}
	}

	return results
}
