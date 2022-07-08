// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// router routes logs, metrics and traces using the configured attributes and
// attribute sources.
// Upon routing it also groups the logs, metrics and spans into a joint upper level
// structure (plog.Logs, pmetric.Metrics and ptrace.Traces respectively) in order
// to not cause higher CPU usage in the exporters when exproting data (it's always
// better to batch before exporting).
type router struct {
	config    Config
	logger    *zap.Logger
	extractor extractor

	defaultLogsExporters    []component.LogsExporter
	logsExporters           map[string][]component.LogsExporter
	defaultMetricsExporters []component.MetricsExporter
	metricsExporters        map[string][]component.MetricsExporter
	defaultTracesExporters  []component.TracesExporter
	tracesExporters         map[string][]component.TracesExporter
}

func newRouter(config Config, logger *zap.Logger) *router {
	return &router{
		config:           config,
		logger:           logger,
		extractor:        newExtractor(config.FromAttribute, logger),
		logsExporters:    make(map[string][]component.LogsExporter),
		metricsExporters: make(map[string][]component.MetricsExporter),
		tracesExporters:  make(map[string][]component.TracesExporter),
	}
}

type routedMetrics struct {
	metrics   pmetric.Metrics
	exporters []component.MetricsExporter
}

func (r *router) RouteMetrics(ctx context.Context, tm pmetric.Metrics) []routedMetrics {
	switch r.config.AttributeSource {
	case resourceAttributeSource:
		return r.routeMetricsForResource(ctx, tm)
	case contextAttributeSource:
		fallthrough
	default:
		return []routedMetrics{r.routeMetricsForContext(ctx, tm)}
	}
}

func (r *router) removeRoutingAttribute(resource pcommon.Resource) {
	resource.Attributes().Remove(r.config.FromAttribute)
}

func (r *router) routeMetricsForResource(_ context.Context, tm pmetric.Metrics) []routedMetrics {
	// routingEntry is used to group pmetric.ResourceMetrics that are routed to
	// the same set of exporters.
	// This way we're not ending up with all the metrics split up which would cause
	// higher CPU usage.
	type routingEntry struct {
		exporters  []component.MetricsExporter
		resMetrics pmetric.ResourceMetricsSlice
	}
	routingMap := map[string]routingEntry{}

	resMetricsSlice := tm.ResourceMetrics()
	for i := 0; i < resMetricsSlice.Len(); i++ {
		resMetrics := resMetricsSlice.At(i)

		attrValue := r.extractor.extractAttrFromResource(resMetrics.Resource())
		exp := r.defaultMetricsExporters
		// If we have an exporter list defined for that attribute value then use it.
		if e, ok := r.metricsExporters[attrValue]; ok {
			exp = e
			if r.config.DropRoutingResourceAttribute {
				r.removeRoutingAttribute(resMetrics.Resource())
			}
		}

		if rEntry, ok := routingMap[attrValue]; ok {
			resMetrics.MoveTo(rEntry.resMetrics.AppendEmpty())
		} else {
			new := pmetric.NewResourceMetricsSlice()
			resMetrics.MoveTo(new.AppendEmpty())

			routingMap[attrValue] = routingEntry{
				exporters:  exp,
				resMetrics: new,
			}
		}
	}

	// Now that we have all the ResourceMetrics grouped, let's create pmetric.Metrics
	// for each group and add it to the returned routedMetrics slice.
	ret := make([]routedMetrics, 0, len(routingMap))
	for _, rEntry := range routingMap {
		metrics := pmetric.NewMetrics()
		metrics.ResourceMetrics().EnsureCapacity(rEntry.resMetrics.Len())
		rEntry.resMetrics.MoveAndAppendTo(metrics.ResourceMetrics())

		ret = append(ret, routedMetrics{
			metrics:   metrics,
			exporters: rEntry.exporters,
		})
	}

	return ret
}

func (r *router) routeMetricsForContext(ctx context.Context, tm pmetric.Metrics) routedMetrics {
	value := r.extractor.extractFromContext(ctx)

	exp, ok := r.metricsExporters[value]
	if !ok {
		return routedMetrics{
			metrics:   tm,
			exporters: r.defaultMetricsExporters,
		}
	}

	return routedMetrics{
		metrics:   tm,
		exporters: exp,
	}
}

type routedTraces struct {
	traces    ptrace.Traces
	exporters []component.TracesExporter
}

func (r *router) RouteTraces(ctx context.Context, tr ptrace.Traces) []routedTraces {
	switch r.config.AttributeSource {
	case resourceAttributeSource:
		return r.routeTracesForResource(ctx, tr)
	case contextAttributeSource:
		fallthrough
	default:
		return []routedTraces{r.routeTracesForContext(ctx, tr)}
	}
}

func (r *router) routeTracesForResource(_ context.Context, tr ptrace.Traces) []routedTraces {
	// routingEntry is used to group ptrace.ResourceSpans that are routed to
	// the same set of exporters.
	// This way we're not ending up with all the logs split up which would cause
	// higher CPU usage.
	type routingEntry struct {
		exporters []component.TracesExporter
		resSpans  ptrace.ResourceSpansSlice
	}
	routingMap := map[string]routingEntry{}

	resSpansSlice := tr.ResourceSpans()
	for i := 0; i < resSpansSlice.Len(); i++ {
		resSpans := resSpansSlice.At(i)

		attrValue := r.extractor.extractAttrFromResource(resSpans.Resource())
		exp := r.defaultTracesExporters
		// If we have an exporter list defined for that attribute value then use it.
		if e, ok := r.tracesExporters[attrValue]; ok {
			exp = e
			if r.config.DropRoutingResourceAttribute {
				r.removeRoutingAttribute(resSpans.Resource())
			}
		}

		if rEntry, ok := routingMap[attrValue]; ok {
			resSpans.MoveTo(rEntry.resSpans.AppendEmpty())
		} else {
			new := ptrace.NewResourceSpansSlice()
			resSpans.MoveTo(new.AppendEmpty())

			routingMap[attrValue] = routingEntry{
				exporters: exp,
				resSpans:  new,
			}
		}
	}

	// Now that we have all the ResourceSpans grouped, let's create ptrace.Traces
	// for each group and add it to the returned routedTraces slice.
	ret := make([]routedTraces, 0, len(routingMap))
	for _, rEntry := range routingMap {
		traces := ptrace.NewTraces()
		traces.ResourceSpans().EnsureCapacity(rEntry.resSpans.Len())
		rEntry.resSpans.MoveAndAppendTo(traces.ResourceSpans())

		ret = append(ret, routedTraces{
			traces:    traces,
			exporters: rEntry.exporters,
		})
	}

	return ret
}

func (r *router) routeTracesForContext(ctx context.Context, tr ptrace.Traces) routedTraces {
	value := r.extractor.extractFromContext(ctx)

	exp, ok := r.tracesExporters[value]
	if !ok {
		return routedTraces{
			traces:    tr,
			exporters: r.defaultTracesExporters,
		}
	}

	return routedTraces{
		traces:    tr,
		exporters: exp,
	}
}

type routedLogs struct {
	logs      plog.Logs
	exporters []component.LogsExporter
}

func (r *router) RouteLogs(ctx context.Context, tl plog.Logs) []routedLogs {
	switch r.config.AttributeSource {
	case resourceAttributeSource:
		return r.routeLogsForResource(ctx, tl)
	case contextAttributeSource:
		fallthrough
	default:
		return []routedLogs{r.routeLogsForContext(ctx, tl)}
	}
}

func (r *router) routeLogsForResource(_ context.Context, tl plog.Logs) []routedLogs {
	// routingEntry is used to group plog.ResourceLogs that are routed to
	// the same set of exporters.
	// This way we're not ending up with all the logs split up which would cause
	// higher CPU usage.
	type routingEntry struct {
		exporters []component.LogsExporter
		resLogs   plog.ResourceLogsSlice
	}
	routingMap := map[string]routingEntry{}

	resLogsSlice := tl.ResourceLogs()
	for i := 0; i < resLogsSlice.Len(); i++ {
		resLogs := resLogsSlice.At(i)

		attrValue := r.extractor.extractAttrFromResource(resLogs.Resource())
		exp := r.defaultLogsExporters
		// If we have an exporter list defined for that attribute value then use it.
		if e, ok := r.logsExporters[attrValue]; ok {
			exp = e
			if r.config.DropRoutingResourceAttribute {
				r.removeRoutingAttribute(resLogs.Resource())
			}
		}

		if rEntry, ok := routingMap[attrValue]; ok {
			resLogs.MoveTo(rEntry.resLogs.AppendEmpty())
		} else {
			new := plog.NewResourceLogsSlice()
			resLogs.MoveTo(new.AppendEmpty())

			routingMap[attrValue] = routingEntry{
				exporters: exp,
				resLogs:   new,
			}
		}
	}

	// Now that we have all the ResourceLogs grouped, let's create plog.Logs
	// for each group and add it to the returned routedLogs slice.
	ret := make([]routedLogs, 0, len(routingMap))
	for _, rEntry := range routingMap {
		logs := plog.NewLogs()
		logs.ResourceLogs().EnsureCapacity(rEntry.resLogs.Len())
		rEntry.resLogs.MoveAndAppendTo(logs.ResourceLogs())

		ret = append(ret, routedLogs{
			logs:      logs,
			exporters: rEntry.exporters,
		})
	}

	return ret
}

func (r *router) routeLogsForContext(ctx context.Context, tl plog.Logs) routedLogs {
	value := r.extractor.extractFromContext(ctx)

	exp, ok := r.logsExporters[value]
	if !ok {
		return routedLogs{
			logs:      tl,
			exporters: r.defaultLogsExporters,
		}
	}

	return routedLogs{
		logs:      tl,
		exporters: exp,
	}
}

// registerExporters registers the exporters as per the configured routing table
// taking into account the provided map of available exporters.
func (r *router) registerExporters(hostExporters map[config.DataType]map[config.ComponentID]component.Exporter) error {
	for _, reg := range []struct {
		registerFunc func(map[config.ComponentID]component.Exporter) error
		typ          config.Type
	}{
		{
			r.registerTracesExporters,
			config.TracesDataType,
		},
		{
			r.registerMetricsExporters,
			config.MetricsDataType,
		},
		{
			r.registerLogsExporters,
			config.LogsDataType,
		},
	} {
		if err := reg.registerFunc(hostExporters[reg.typ]); err != nil {
			if errors.Is(err, errDefaultExporterNotFound) || errors.Is(err, errExporterNotFound) {
				r.logger.Warn("can't find the exporter for the routing processor for this pipeline type. This is OK if you did not specify this processor for that pipeline type",
					zap.Any("pipeline_type", reg.typ),
					zap.Error(err),
				)
			} else {
				// this seems to be more serious than what expected
				return err
			}
		}
	}

	if len(r.defaultLogsExporters) == 0 &&
		len(r.defaultMetricsExporters) == 0 &&
		len(r.defaultTracesExporters) == 0 &&
		len(r.logsExporters) == 0 &&
		len(r.metricsExporters) == 0 &&
		len(r.tracesExporters) == 0 {
		return errNoExportersAfterRegistration
	}

	return nil
}

// ExporterMap represents a maping from exporter name to the actual exporter object.
type ExporterMap map[string]component.Exporter

func (r *router) registerMetricsExporters(hostMetricsExporters map[config.ComponentID]component.Exporter) error {
	if len(hostMetricsExporters) == 0 {
		return nil
	}

	availableExporters := ExporterMap{}
	for compID, exp := range hostMetricsExporters {
		mExp, ok := exp.(component.MetricsExporter)
		if !ok {
			return fmt.Errorf("the exporter %q isn't a metrics exporter", compID.String())
		}
		availableExporters[compID.String()] = mExp
	}

	return r.registerExportersForRoutes(availableExporters)
}

func (r *router) registerLogsExporters(hostLogsExporters map[config.ComponentID]component.Exporter) error {
	if len(hostLogsExporters) == 0 {
		return nil
	}

	availableExporters := ExporterMap{}
	for compID, exp := range hostLogsExporters {
		mExp, ok := exp.(component.LogsExporter)
		if !ok {
			return fmt.Errorf("the exporter %q isn't a logs exporter", compID.String())
		}
		availableExporters[compID.String()] = mExp
	}

	return r.registerExportersForRoutes(availableExporters)
}

func (r *router) registerTracesExporters(hostTracesExporters map[config.ComponentID]component.Exporter) error {
	if len(hostTracesExporters) == 0 {
		return nil
	}

	availableExporters := ExporterMap{}
	for compID, exp := range hostTracesExporters {
		tExp, ok := exp.(component.TracesExporter)
		if !ok {
			return fmt.Errorf("the exporter %q isn't a traces exporter", compID.String())
		}
		availableExporters[compID.String()] = tExp
	}

	return r.registerExportersForRoutes(availableExporters)
}

// registerExportersForRoutes registers exporters according to the configuring
// routing table, taking into account the provided map of available exporters.
func (r *router) registerExportersForRoutes(available ExporterMap) error {
	r.logger.Debug("Registering exporters for routes")

	// default exporters
	if err := r.registerExportersForDefaultRoute(available); err != nil {
		return err
	}

	// exporters for each defined value
	for _, item := range r.config.Table {
		if err := r.registerExportersForRoute(item.Value, available, item.Exporters); err != nil {
			return err
		}
	}

	return nil
}

// registerExportersForDefaultRoute registers the configured default exporters
// using the provided available exporters map.
func (r *router) registerExportersForDefaultRoute(available ExporterMap) error {
	for _, exp := range r.config.DefaultExporters {
		v, ok := available[exp]
		if !ok {
			return fmt.Errorf("error registering default exporter %q: %w",
				exp, errDefaultExporterNotFound,
			)
		}

		switch exp := v.(type) {
		case component.TracesExporter:
			r.defaultTracesExporters = append(r.defaultTracesExporters, exp)
		case component.MetricsExporter:
			r.defaultMetricsExporters = append(r.defaultMetricsExporters, exp)
		case component.LogsExporter:
			r.defaultLogsExporters = append(r.defaultLogsExporters, exp)
		default:
			return fmt.Errorf("unknown exporter type %T", v)
		}
	}

	return nil
}

// registerExportersForRoute registers the requested exporters using the provided
// available exporters map to check if they were available.
func (r *router) registerExportersForRoute(route string, available ExporterMap, requested []string) error {
	r.logger.Debug("Registering exporter for route",
		zap.String("route", route),
		zap.Any("requested", requested),
	)

	for _, exp := range requested {
		v, ok := available[exp]
		if !ok {
			return fmt.Errorf("error registering route %q for exporter %q: %w",
				route, exp, errExporterNotFound,
			)
		}

		switch exp := v.(type) {
		case component.TracesExporter:
			r.tracesExporters[route] = append(r.tracesExporters[route], exp)
		case component.MetricsExporter:
			r.metricsExporters[route] = append(r.metricsExporters[route], exp)
		case component.LogsExporter:
			r.logsExporters[route] = append(r.logsExporters[route], exp)
		default:
			return fmt.Errorf("unknown exporter type %T", v)
		}
	}

	return nil
}
