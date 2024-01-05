// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
)

var _ exporter.Logs = (*logExporterImp)(nil)

type logExporterImp struct {
	loadBalancer loadBalancer
	resourceKeys []string

	logConsumer logConsumer
	started     bool
	shutdownWg  sync.WaitGroup
}

type routingEntry struct {
	routingKey routingKey
	keyValue   string
	log        plog.Logs
}

type logConsumer func(ctx context.Context, td plog.Logs) error

// Create new logs exporter
func newLogsExporter(params exporter.CreateSettings, cfg component.Config) (*logExporterImp, error) {
	exporterFactory := otlpexporter.NewFactory()

	lb, err := newLoadBalancer(params, cfg, func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := buildExporterConfig(cfg.(*Config), endpoint)
		return exporterFactory.CreateLogsExporter(ctx, params, &oCfg)
	})
	if err != nil {
		return nil, err
	}

	logExporter := logExporterImp{loadBalancer: lb}

	switch cfg.(*Config).RoutingKey {
	case "service":
		logExporter.logConsumer = logExporter.consumeLogsByResource
		logExporter.resourceKeys = []string{"service.name"}
	case "traceID", "":
		logExporter.logConsumer = logExporter.consumeLogsById
	case "resource":
		logExporter.resourceKeys = cfg.(*Config).ResourceKeys
		logExporter.logConsumer = logExporter.consumeLogsByResource
	default:
		return nil, fmt.Errorf("unsupported routing_key: %s", cfg.(*Config).RoutingKey)
	}
	return &logExporter, nil
}

func (e *logExporterImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *logExporterImp) Start(ctx context.Context, host component.Host) error {
	e.started = true
	return e.loadBalancer.Start(ctx, host)
}

func (e *logExporterImp) Shutdown(context.Context) error {
	if !e.started {
		return nil
	}
	e.started = false
	e.shutdownWg.Wait()
	return nil
}

func (e *logExporterImp) ConsumeLogs(ctx context.Context, td plog.Logs) error {
	return e.logConsumer(ctx, td)
}

func (e *logExporterImp) consumeLog(ctx context.Context, td plog.Logs, rid string) error {
	// Routes a single log via a given routing ID
	endpoint := e.loadBalancer.Endpoint([]byte(rid))
	exp, err := e.loadBalancer.Exporter(endpoint)
	if err != nil {
		return err
	}

	te, ok := exp.(exporter.Logs)
	if !ok {
		return fmt.Errorf("unable to export logs, unexpected exporter type: expected exporter.Logs but got %T", exp)
	}

	start := time.Now()
	err = te.ConsumeLogs(ctx, td)
	duration := time.Since(start)

	if err == nil {
		_ = stats.RecordWithTags(
			ctx,
			[]tag.Mutator{tag.Upsert(endpointTagKey, endpoint), successTrueMutator},
			mBackendLatency.M(duration.Milliseconds()))
	} else {
		_ = stats.RecordWithTags(
			ctx,
			[]tag.Mutator{tag.Upsert(endpointTagKey, endpoint), successFalseMutator},
			mBackendLatency.M(duration.Milliseconds()))
	}
	return err
}

func random() pcommon.TraceID {
	v1 := uint8(rand.Intn(256))
	v2 := uint8(rand.Intn(256))
	v3 := uint8(rand.Intn(256))
	v4 := uint8(rand.Intn(256))
	return [16]byte{v1, v2, v3, v4}
}

func (e *logExporterImp) consumeLogsById(ctx context.Context, td plog.Logs) error {
	var errs error
	batches := batchpersignal.SplitLogs(td)

	for _, t := range batches {
		if tid, err := routeByTraceId(t); err == nil {
			errs = multierr.Append(errs, e.consumeLog(ctx, t, tid))
		} else {
			return err
		}
	}
	return errs
}

func (e *logExporterImp) consumeLogsByResource(ctx context.Context, td plog.Logs) error {
	var errs error
	routeBatches, err := splitLogsByResourceAttr(td, e.resourceKeys)
	if err != nil {
		return err
	}
	for _, batch := range routeBatches {
		switch batch.routingKey {
		case resourceRouting:
			errs = multierr.Append(errs, e.consumeLog(ctx, batch.log, batch.keyValue))
		case traceIDRouting:
			errs = multierr.Append(errs, e.consumeLogsById(ctx, batch.log))
		}
	}
	return errs

}

func getResourceAttrValue(rs plog.ResourceLogs, resourceKeys []string) (string, bool) {
	found := false
	if len(resourceKeys) == 0 {
		for k := range rs.Resource().Attributes().AsRaw() {
			resourceKeys = append(resourceKeys, k)
		}
		sort.Strings(resourceKeys)
	}

	attrsHash := getResourceAttrValues(rs.Resource().Attributes(), resourceKeys)
	if len(attrsHash) > 0 { // as long as one of attribute exist
		found = true
	}
	res := strings.Join(attrsHash, "")
	return res, found
}

func getResourceAttrValues(attrs pcommon.Map, resourceKeys []string) []string {
	attrsHash := make([]string, 0)
	for _, k := range resourceKeys {
		if v, ok := attrs.Get(k); ok {
			attrsHash = append(attrsHash, v.AsString())
		}
	}
	return attrsHash
}

func splitLogsByResourceAttr(batches plog.Logs, resourceKeys []string) ([]routingEntry, error) {
	// This function batches all the ResourceLogs with the same routing resource attribute value into a single plog.Log
	// This returns a list of routing entries which consists of the routing key, routing key value and the log
	// There should be a 1:1 mapping between key value <-> log
	// This is because we group all Resource Logs with the same key value under a single log
	var result []routingEntry
	rss := batches.ResourceLogs()

	// This is a mapping between the resource attribute values found and the constructed log
	routeMap := make(map[string]plog.Logs)

	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		if keyValue, ok := getResourceAttrValue(rs, resourceKeys); ok {
			// Check if this keyValue has previously been seen
			// if not it constructs an empty plog.Logs
			if _, ok := routeMap[keyValue]; !ok {
				routeMap[keyValue] = plog.NewLogs()
			}
			rs.CopyTo(routeMap[keyValue].ResourceLogs().AppendEmpty())
		} else {
			// If none of the resource attributes have been found
			// We fallback to routing the given Resource Span by Trace ID
			t := plog.NewLogs()
			rs.CopyTo(t.ResourceLogs().AppendEmpty())
			// We can't route this whole Resource Span by a single trace ID
			// because it's possible for the spans under the RS to have different trace IDs
			result = append(result, routingEntry{
				routingKey: traceIDRouting,
				log:        t,
			})
		}
	}

	// We convert the attr value:log mapping into a list of routingEntries
	for key, log := range routeMap {
		result = append(result, routingEntry{
			routingKey: resourceRouting,
			keyValue:   key,
			log:        log,
		})
	}

	return result, nil
}

func routeByTraceId(td plog.Logs) (string, error) {
	// This function assumes that you are receiving a single log i.e. single traceId
	// returns the traceId as the routing key
	rs := td.ResourceLogs()
	if rs.Len() == 0 {
		return "", errors.New("empty resource logs")
	}
	if rs.Len() > 1 {
		return "", errors.New("routeByTraceId must receive a plog.Logs with a single ResourceLog")
	}
	ils := rs.At(0).ScopeLogs()
	if ils.Len() == 0 {
		return "", errors.New("empty scope logs")
	}
	logs := ils.At(0).LogRecords()
	if logs.Len() == 0 {
		return "", errors.New("empty logs")
	}
	tid := logs.At(0).TraceID()
	return string(tid[:]), nil
}
