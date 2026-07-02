// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/metric"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
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
	case traceIDRoutingStr:
		logExporter.routingKey = traceIDRouting
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
	case traceIDRouting:
		batches = splitLogsByTraceID(ld)
	case svcRouting:
		batches = splitLogsByServiceName(ld)
	case resourceRouting:
		batches = splitLogsByResourceID(ld)
	case attrRouting:
		batches = splitLogsByAttributes(ld, e.routingAttrs)
	}

	logsByExporter := make(map[*wrappedExporter]plog.Logs, len(batches))
	exporterEndpoints := make(map[*wrappedExporter]string, len(batches))

	for routingID, lds := range batches {
		exp, endpoint, err := e.loadBalancer.exporterAndEndpoint([]byte(routingID))
		if err != nil {
			return err
		}

		_, ok := logsByExporter[exp]
		if !ok {
			exp.consumeWG.Add(1)
			logsByExporter[exp] = lds
			exporterEndpoints[exp] = endpoint
		} else {
			mergeLogs(logsByExporter[exp], lds)
		}
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

func splitLogsByServiceName(ld plog.Logs) map[string]plog.Logs {
	results := make(map[string]plog.Logs, ld.ResourceLogs().Len())

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rm := ld.ResourceLogs().At(i)

		key := ""
		svc, ok := rm.Resource().Attributes().Get(string(conventions.ServiceNameKey))
		if ok {
			key = svc.Str()
		}

		existing, ok := results[key]
		if ok {
			rm.CopyTo(existing.ResourceLogs().AppendEmpty())
		} else {
			newLD := plog.NewLogs()
			rm.CopyTo(newLD.ResourceLogs().AppendEmpty())
			results[key] = newLD
		}
	}

	return results
}

func splitLogsByResourceID(ld plog.Logs) map[string]plog.Logs {
	results := make(map[string]plog.Logs, ld.ResourceLogs().Len())

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rm := ld.ResourceLogs().At(i)

		key := identity.OfResource(rm.Resource()).String()
		existing, ok := results[key]
		if ok {
			rm.CopyTo(existing.ResourceLogs().AppendEmpty())
		} else {
			newLD := plog.NewLogs()
			rm.CopyTo(newLD.ResourceLogs().AppendEmpty())
			results[key] = newLD
		}
	}

	return results
}

// splitLogsByTraceID splits logs per-record by trace ID, so records with
// different trace IDs within the same ResourceLogs are routed to different
// backends. Records without a trace ID get a random key to avoid hot-spotting
// a single backend.
func splitLogsByTraceID(ld plog.Logs) map[string]plog.Logs {
	results := make(map[string]plog.Logs)

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)

			// Group log records by trace ID within this scope.
			type scopeBatch struct {
				rl plog.ResourceLogs
			}
			batches := make(map[string]*scopeBatch)

			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				traceID := lr.TraceID()
				var key string
				if traceID.IsEmpty() {
					key = randomTraceID().String()
				} else {
					key = traceID.String()
				}

				sb, ok := batches[key]
				if !ok {
					// First record with this trace ID in this scope —
					// create a new ResourceLogs with resource + scope copied.
					var logs plog.Logs
					existing, found := results[key]
					if found {
						logs = existing
					} else {
						logs = plog.NewLogs()
						results[key] = logs
					}
					newRL := logs.ResourceLogs().AppendEmpty()
					rl.Resource().CopyTo(newRL.Resource())
					newRL.SetSchemaUrl(rl.SchemaUrl())
					newSL := newRL.ScopeLogs().AppendEmpty()
					sl.Scope().CopyTo(newSL.Scope())
					newSL.SetSchemaUrl(sl.SchemaUrl())
					sb = &scopeBatch{rl: newRL}
					batches[key] = sb
				}

				tgt := sb.rl.ScopeLogs().At(sb.rl.ScopeLogs().Len() - 1).LogRecords().AppendEmpty()
				lr.CopyTo(tgt)
			}
		}
	}

	return results
}

// splitLogsByAttributes splits logs per-record, building a routing key from
// resource → scope → log record attributes (including pseudo attributes
// log.severity and log.body). This mirrors the metrics exporter pattern where
// routing goes down to the deepest supported element when attributes are not
// fully resolved at higher levels.
func splitLogsByAttributes(ld plog.Logs, attrs []string) map[string]plog.Logs {
	results := make(map[string]plog.Logs)
	var rKey strings.Builder

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		resourceAttrs := rl.Resource().Attributes()

		var baseResourceKeyBuilder strings.Builder
		pendingResourceAttrs := make([]string, 0, len(attrs))
		for _, a := range attrs {
			if val, ok := resourceAttrs.Get(a); ok {
				baseResourceKeyBuilder.WriteString(buildAttributeRoutingKeyValue(a, val))
				continue
			}
			pendingResourceAttrs = append(pendingResourceAttrs, a)
		}
		baseResourceKey := baseResourceKeyBuilder.String()

		if len(pendingResourceAttrs) == 0 || rl.ScopeLogs().Len() == 0 {
			existing, ok := results[baseResourceKey]
			if ok {
				rl.CopyTo(existing.ResourceLogs().AppendEmpty())
			} else {
				newLD := plog.NewLogs()
				rl.CopyTo(newLD.ResourceLogs().AppendEmpty())
				results[baseResourceKey] = newLD
			}
			continue
		}

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			scopeAttrs := sl.Scope().Attributes()

			var baseScopeKeyBuilder strings.Builder
			baseScopeKeyBuilder.WriteString(baseResourceKey)
			pendingScopeAttrs := make([]string, 0, len(pendingResourceAttrs))
			for _, a := range pendingResourceAttrs {
				if val, ok := scopeAttrs.Get(a); ok {
					baseScopeKeyBuilder.WriteString(buildAttributeRoutingKeyValue(a, val))
					continue
				}
				pendingScopeAttrs = append(pendingScopeAttrs, a)
			}
			baseScopeKey := baseScopeKeyBuilder.String()

			if len(pendingScopeAttrs) == 0 {
				appendLogScope(results, baseScopeKey, rl, sl)
				continue
			}

			type recordBatch struct {
				rl plog.ResourceLogs
			}
			batches := make(map[string]*recordBatch)

			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				rKey.Reset()
				rKey.WriteString(baseScopeKey)

				for _, a := range pendingScopeAttrs {
					if a == pseudoAttrLogSeverity {
						rKey.WriteString(buildAttributeRoutingKeyStrValue(a, lr.SeverityText()))
						continue
					}
					if a == pseudoAttrLogBody {
						rKey.WriteString(buildAttributeRoutingKeyStrValue(a, lr.Body().AsString()))
						continue
					}
					if val, ok := lr.Attributes().Get(a); ok {
						rKey.WriteString(buildAttributeRoutingKeyValue(a, val))
					} else {
						rKey.WriteString(buildAttributeRoutingKey(a))
					}
				}

				key := rKey.String()
				rb, ok := batches[key]
				if !ok {
					var logs plog.Logs
					existing, found := results[key]
					if found {
						logs = existing
					} else {
						logs = plog.NewLogs()
						results[key] = logs
					}
					newRL := logs.ResourceLogs().AppendEmpty()
					rl.Resource().CopyTo(newRL.Resource())
					newRL.SetSchemaUrl(rl.SchemaUrl())
					newSL := newRL.ScopeLogs().AppendEmpty()
					sl.Scope().CopyTo(newSL.Scope())
					newSL.SetSchemaUrl(sl.SchemaUrl())
					rb = &recordBatch{rl: newRL}
					batches[key] = rb
				}

				tgt := rb.rl.ScopeLogs().At(rb.rl.ScopeLogs().Len() - 1).LogRecords().AppendEmpty()
				lr.CopyTo(tgt)
			}
		}
	}

	return results
}

func randomTraceID() pcommon.TraceID {
	v1 := uint8(rand.IntN(256))
	v2 := uint8(rand.IntN(256))
	v3 := uint8(rand.IntN(256))
	v4 := uint8(rand.IntN(256))
	return [16]byte{v1, v2, v3, v4}
}

// appendLogScope adds a full scope to the results map under the given key,
// preserving resource and scope structure.
func appendLogScope(results map[string]plog.Logs, key string, rl plog.ResourceLogs, sl plog.ScopeLogs) {
	existing, ok := results[key]
	if !ok {
		existing = plog.NewLogs()
		results[key] = existing
	}
	newRL := existing.ResourceLogs().AppendEmpty()
	rl.Resource().CopyTo(newRL.Resource())
	newRL.SetSchemaUrl(rl.SchemaUrl())
	sl.CopyTo(newRL.ScopeLogs().AppendEmpty())
}
