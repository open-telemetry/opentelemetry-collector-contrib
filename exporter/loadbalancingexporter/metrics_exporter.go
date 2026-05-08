// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/metric"
	conventions "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

var _ exporter.Metrics = (*metricExporterImp)(nil)

const metricBatcherEnqueueBackoff = 2 * time.Millisecond

type metricExporterImp struct {
	loadBalancer *loadBalancer
	batcher      *metricBatcher
	centralQueue *centralQueue
	centralCodec *queuePayloadCodec
	routingKey   routingKey

	logger        *zap.Logger
	started       atomic.Bool
	telemetry     *metadata.TelemetryBuilder
	centralCancel context.CancelFunc
	centralWG     sync.WaitGroup
}

func newMetricsExporter(params exporter.Settings, cfg component.Config) (*metricExporterImp, error) {
	telemetry, err := metadata.NewTelemetryBuilder(params.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	exporterFactory := otlpexporter.NewFactory()
	cfFunc := func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := buildExporterConfig(cfg.(*Config), endpoint)
		oParams := buildExporterSettings(exporterFactory.Type(), params, endpoint)

		return exporterFactory.CreateMetrics(ctx, oParams, &oCfg)
	}

	lb, err := newLoadBalancer(params.Logger, cfg, cfFunc, telemetry)
	if err != nil {
		return nil, err
	}

	metricExporter := metricExporterImp{
		loadBalancer: lb,
		routingKey:   svcRouting,
		telemetry:    telemetry,
		logger:       params.Logger,
	}
	if cfg.(*Config).CentralQueue.Enabled {
		centralCfg := cfg.(*Config).CentralQueue
		centralTelemetry, telemetryErr := newCentralQueueTelemetry(params.TelemetrySettings, signalKindMetrics)
		if telemetryErr != nil {
			return nil, telemetryErr
		}
		metricExporter.centralQueue = newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           centralCfg.MaxCompressedBytes,
			maxInflightUncompressedBytes: centralCfg.MaxInflightUncompressedBytes,
			maxUncompressedBatchBytes:    centralCfg.MaxUncompressedBatchBytes,
			telemetry:                    centralTelemetry,
		})
		metricExporter.centralCodec = newQueuePayloadCodec(centralCfg.PayloadCompression)
	}

	switch cfg.(*Config).RoutingKey {
	case svcRoutingStr, "":
		// default case for empty routing key
		metricExporter.routingKey = svcRouting
	case resourceRoutingStr:
		metricExporter.routingKey = resourceRouting
	case metricNameRoutingStr:
		metricExporter.routingKey = metricNameRouting
	case streamIDRoutingStr:
		metricExporter.routingKey = streamIDRouting
	default:
		return nil, fmt.Errorf("unsupported routing_key: %q", cfg.(*Config).RoutingKey)
	}

	if cfg.(*Config).MetricBatcher.Enabled {
		metricExporter.batcher, err = newMetricBatcher(
			params.Logger,
			params.TelemetrySettings,
			metricBatcherSettings{
				maxDataPoints:            cfg.(*Config).MetricBatcher.MaxDataPoints,
				maxBytes:                 cfg.(*Config).MetricBatcher.MaxBytes,
				flushInterval:            cfg.(*Config).MetricBatcher.FlushInterval,
				maxRetryBufferMultiplier: cfg.(*Config).MetricBatcher.MaxRetryBufferMultiplier,
				payloadCompression:       cfg.(*Config).MetricBatcher.PayloadCompression,
			},
			metricExporter.consumeBatch,
			metricExporter.rerouteDrainBatch,
		)
		if err != nil {
			return nil, err
		}
		lb.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
			return metricExporter.batcher.Remove(ctx, endpoint, exp)
		}
	}

	return &metricExporter, nil
}

func (*metricExporterImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (e *metricExporterImp) Start(ctx context.Context, host component.Host) error {
	if err := e.loadBalancer.Start(ctx, host); err != nil {
		return err
	}
	e.started.Store(true)
	if e.centralQueue != nil {
		dispatchCtx, cancel := context.WithCancel(context.Background())
		e.centralCancel = cancel
		e.centralWG.Add(1)
		go e.runCentralQueue(dispatchCtx)
	}
	return nil
}

func (e *metricExporterImp) Shutdown(ctx context.Context) error {
	if !e.started.Swap(false) {
		return nil
	}
	var err error
	if e.batcher != nil {
		err = e.batcher.Shutdown(ctx)
	}
	if e.centralQueue != nil {
		e.centralQueue.stop()
		if e.centralCancel != nil {
			e.centralCancel()
		}
		waitCtx, cancel := context.WithTimeout(ctx, time.Second)
		waitErr := waitForInflight(waitCtx, &e.centralWG)
		cancel()
		err = errors.Join(err, waitErr)
		if waitErr != nil {
			return err
		}
		err = errors.Join(err, e.centralCodec.Close())
	}
	err = errors.Join(err, e.loadBalancer.Shutdown(ctx))
	return err
}

func (e *metricExporterImp) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if !e.started.Load() {
		return errExporterIsStopping
	}

	batches, err := e.splitMetricsByRouting(md)
	if err != nil {
		return err
	}

	if e.centralQueue != nil {
		return e.consumeMetricsCentralQueue(batches)
	}

	if e.batcher != nil {
		endpointBatches, err := e.groupRoutedMetricsByEndpoint(batches)
		if err != nil {
			return err
		}
		return e.enqueueEndpointBatches(ctx, endpointBatches, true)
	}

	return e.consumeMetricsByExporter(ctx, batches)
}

func (e *metricExporterImp) consumeMetricsCentralQueue(batches map[string]pmetric.Metrics) error {
	var errs error
	now := time.Now()
	for routingKey, md := range batches {
		item, err := newCentralQueueMetricsItem([]byte(routingKey), md, e.centralCodec, now)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		errs = multierr.Append(errs, e.centralQueue.enqueue(item))
	}
	return errs
}

func (e *metricExporterImp) runCentralQueue(ctx context.Context) {
	defer e.centralWG.Done()
	for {
		lease, err := e.centralQueue.lease(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, errCentralQueueStopped) {
				return
			}
			if errors.Is(err, errCentralQueueInflightFull) {
				select {
				case <-ctx.Done():
					return
				case <-time.After(centralQueueLeasePollInterval):
					continue
				}
			}
			e.logger.Warn("failed to lease central metric queue item", zap.Error(err))
			continue
		}

		err = e.consumeCentralQueueMetricItem(ctx, lease.item)
		if err == nil {
			lease.done()
			continue
		}
		if ctx.Err() != nil {
			lease.done()
			return
		}
		if consumererror.IsPermanent(err) {
			e.logger.Warn("dropping central metric queue item after permanent export error", zap.Error(err))
			lease.done()
			continue
		}
		item := lease.item
		nextAttempt := time.Now().Add(centralQueueRetryDelay(item.attempt))
		item.attempt++
		lease.item = item
		if requeueErr := lease.requeue(nextAttempt); requeueErr != nil {
			e.logger.Warn("failed to requeue central metric queue item", zap.Error(requeueErr), zap.Error(err))
		}
	}
}

func (e *metricExporterImp) consumeCentralQueueMetricItem(ctx context.Context, item centralQueueItem) error {
	md, err := decodeCentralQueueMetricsItem(item, e.centralCodec)
	if err != nil {
		e.logger.Warn("dropping invalid central metric queue payload", zap.Error(err))
		return nil
	}
	exp, _, err := e.loadBalancer.exporterAndEndpoint(item.routingKey)
	if err != nil {
		return err
	}

	exp.forceStartConsume()
	defer exp.doneConsume()

	recordMetricBackendRequest(ctx, e.telemetry, exp.metricRequestAttr, md)
	start := time.Now()
	err = exp.ConsumeMetrics(ctx, md)
	duration := time.Since(start)
	e.recordBackendResultWithoutDrain(ctx, exp, duration, err, true)
	return err
}

type endpointMetricsBatch struct {
	metrics pmetric.Metrics
	exp     *wrappedExporter
}

func (e *metricExporterImp) splitMetricsByRouting(md pmetric.Metrics) (map[string]pmetric.Metrics, error) {
	var batches map[string]pmetric.Metrics

	switch e.routingKey {
	case svcRouting:
		var errs []error
		batches, errs = splitMetricsByResourceServiceName(md)
		if len(errs) > 0 {
			for _, ee := range errs {
				e.logger.Error("failed to export metric", zap.Error(ee))
			}
			if len(batches) == 0 {
				return nil, consumererror.NewPermanent(errors.Join(errs...))
			}
		}
	case resourceRouting:
		batches = splitMetricsByResourceID(md)
	case metricNameRouting:
		batches = splitMetricsByMetricName(md)
	case streamIDRouting:
		batches = splitMetricsByStreamID(md)
	}

	return batches, nil
}

func (e *metricExporterImp) groupRoutedMetricsByEndpoint(
	batches map[string]pmetric.Metrics,
) (map[string]*endpointMetricsBatch, error) {
	endpointBatches := make(map[string]*endpointMetricsBatch)

	for routingID, mds := range batches {
		exp, endpoint, err := e.loadBalancer.exporterAndEndpoint([]byte(routingID))
		if err != nil {
			return endpointBatches, err
		}

		ep := endpointWithPort(endpoint)
		batch, ok := endpointBatches[ep]
		if !ok {
			batch = &endpointMetricsBatch{metrics: pmetric.NewMetrics(), exp: exp}
			endpointBatches[ep] = batch
		}
		batch.exp = exp
		mergeMetricChunksByMove(batch.metrics, mds)
	}

	return endpointBatches, nil
}

func (e *metricExporterImp) enqueueEndpointBatches(
	ctx context.Context,
	batches map[string]*endpointMetricsBatch,
	retryOnStopping bool,
) error {
	var errs error
	failed := pmetric.NewMetrics()
	if len(batches) == 0 {
		return nil
	}

	pending := make(map[string]*endpointMetricsBatch, len(batches))
	maps.Copy(pending, batches)

	for len(pending) > 0 {
		madeProgress := false

		for ep, batch := range pending {
			// TryEnqueue is non-blocking; false,nil means queue is currently full.
			// On enqueued=true, ownership of batch.metrics is transferred to the batcher.
			// Do not read or mutate batch.metrics after successful enqueue.
			enqueued, err := e.batcher.TryEnqueue(ep, batch.exp, batch.metrics)

			if errors.Is(err, errMetricBatcherExporterStopping) && retryOnStopping {
				// Reroute once when endpoint exporter is stopping.
				rerouted, rerouteErr := e.splitMetricsByRouting(batch.metrics)
				errs = multierr.Append(errs, rerouteErr)
				if rerouteErr != nil {
					metrics.Merge(failed, batch.metrics)
				} else {
					reroutedBatches, groupErr := e.groupRoutedMetricsByEndpoint(rerouted)
					errs = multierr.Append(errs, groupErr)
					if groupErr != nil {
						for _, batch := range reroutedBatches {
							metrics.Merge(failed, batch.metrics)
						}
						for _, mds := range rerouted {
							metrics.Merge(failed, mds)
						}
					} else {
						rerouteErr = e.enqueueEndpointBatches(ctx, reroutedBatches, false)
						if rerouteErr != nil {
							var mErr consumererror.Metrics
							if errors.As(rerouteErr, &mErr) {
								metrics.Merge(failed, mErr.Data())
								rerouteErr = errors.Unwrap(mErr)
							}
							errs = multierr.Append(errs, rerouteErr)
						}
					}
				}
				delete(pending, ep)
				continue
			}
			if err != nil {
				errs = multierr.Append(errs, err)
				metrics.Merge(failed, batch.metrics)
				delete(pending, ep)
				continue
			}
			if enqueued {
				madeProgress = true
				delete(pending, ep)
			}
		}

		if len(pending) == 0 {
			break
		}

		if madeProgress {
			continue
		}

		// Back off when all pending endpoint queues are currently full.
		timer := time.NewTimer(metricBatcherEnqueueBackoff)
		select {
		case <-ctx.Done():
			// If context was canceled or timed out we send out failed & pending metrics for a retry
			for _, batch := range pending {
				metrics.Merge(failed, batch.metrics)
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			errs = multierr.Append(errs, ctx.Err())
			if failed.DataPointCount() > 0 {
				return consumererror.NewMetrics(errs, failed)
			}
			return errs
		case <-timer.C:
		}
	}

	if failed.DataPointCount() > 0 {
		return consumererror.NewMetrics(errs, failed)
	}

	return errs
}

func (e *metricExporterImp) consumeMetricsByExporter(
	ctx context.Context,
	batches map[string]pmetric.Metrics,
) error {
	return e.consumeMetricsByExporterAttempt(ctx, batches, 0)
}

func (e *metricExporterImp) consumeMetricsByExporterAttempt(
	ctx context.Context,
	batches map[string]pmetric.Metrics,
	rerouteAttempt int,
) error {
	// Now assign each batch to an exporter, and merge as we go
	metricsByExporter := map[*wrappedExporter]pmetric.Metrics{}
	needsCleanup := true
	defer func() {
		if !needsCleanup {
			return
		}
		for exp := range metricsByExporter {
			exp.doneConsume()
		}
	}()

	for routingID, mds := range batches {
		exp, _, err := e.loadBalancer.exporterAndEndpoint([]byte(routingID))
		if err != nil {
			return err
		}

		expMetrics, ok := metricsByExporter[exp]
		if !ok {
			if !exp.tryStartConsume() {
				return errExporterIsStopping
			}
			expMetrics = pmetric.NewMetrics()
			metricsByExporter[exp] = expMetrics
		}

		metrics.Merge(expMetrics, mds)
	}

	needsCleanup = false
	var errs error
	for exp, mds := range metricsByExporter {
		var retryMetrics pmetric.Metrics
		var failedMetrics pmetric.Metrics
		if directRerouteAttemptAllowed(e.loadBalancer, rerouteAttempt) {
			retryMetrics = pmetric.NewMetrics()
			mds.CopyTo(retryMetrics)
			failedMetrics = pmetric.NewMetrics()
			mds.CopyTo(failedMetrics)
		}
		recordMetricBackendRequest(ctx, e.telemetry, exp.metricRequestAttr, mds)
		start := time.Now()
		err := exp.ConsumeMetrics(ctx, mds)
		duration := time.Since(start)

		exp.doneConsume()
		decision := e.recordBackendResult(ctx, exp, duration, err, true)
		if err != nil && shouldRerouteDirectFailure(e.loadBalancer, exp.endpoint, decision, rerouteAttempt) {
			rerouted, splitErr := e.splitMetricsByRouting(retryMetrics)
			if splitErr != nil {
				e.loadBalancer.recordBackendReroute(ctx, "metrics", decision.reason, splitErr)
				err = consumererror.NewMetrics(splitErr, failedMetrics)
			} else {
				rerouteErr := e.consumeMetricsByExporterAttempt(ctx, rerouted, rerouteAttempt+1)
				e.loadBalancer.recordBackendReroute(ctx, "metrics", decision.reason, rerouteErr)
				err = wrapDirectMetricsRerouteError(rerouteErr, failedMetrics)
			}
		}
		errs = multierr.Append(errs, err)
	}

	return errs
}

func (e *metricExporterImp) consumeBatch(ctx context.Context, we *wrappedExporter, md pmetric.Metrics, reason string) error {
	var retryMetrics pmetric.Metrics
	retryAllowed := reason != metricFlushReasonShutdown && directRerouteAttemptAllowed(e.loadBalancer, 0)
	if retryAllowed {
		retryMetrics = pmetric.NewMetrics()
		md.CopyTo(retryMetrics)
	}
	if reason == metricFlushReasonResolverChange || reason == metricFlushReasonShutdown {
		we.forceStartConsume()
	} else if !we.tryStartConsume() {
		if retryAllowed {
			return metricBatcherRerouteableError{
				err:  errMetricBatcherExporterStopping,
				data: retryMetrics,
				recordReroute: func(ctx context.Context, rerouteErr error) {
					e.loadBalancer.recordBackendReroute(ctx, "metrics", endpointFailureExporterStopping, rerouteErr)
				},
			}
		}
		return errMetricBatcherExporterStopping
	}
	defer we.doneConsume()

	recordMetricBackendRequest(ctx, e.telemetry, we.metricRequestAttr, md)
	start := time.Now()
	err := we.ConsumeMetrics(ctx, md)
	duration := time.Since(start)
	decision := e.recordBackendResultHealthOnly(ctx, we, duration, err, reason != metricFlushReasonShutdown)
	if err != nil && shouldRerouteDirectFailure(e.loadBalancer, we.endpoint, decision, 0) {
		e.loadBalancer.cleanupBackendWithoutDrain(ctx, we.endpoint)
		return metricBatcherRerouteableError{
			err:  err,
			data: retryMetrics,
			recordReroute: func(ctx context.Context, rerouteErr error) {
				e.loadBalancer.recordBackendReroute(ctx, "metrics", decision.reason, rerouteErr)
			},
		}
	}

	return err
}

type metricBatcherRerouteableError struct {
	err           error
	data          pmetric.Metrics
	recordReroute func(context.Context, error)
}

func (e metricBatcherRerouteableError) Error() string {
	return e.err.Error()
}

func (e metricBatcherRerouteableError) Unwrap() error {
	return e.err
}

func (e metricBatcherRerouteableError) Data() pmetric.Metrics {
	return e.data
}

func (e metricBatcherRerouteableError) RecordReroute(ctx context.Context, err error) {
	if e.recordReroute != nil {
		e.recordReroute(ctx, err)
	}
}

func (e *metricExporterImp) rerouteDrainBatch(ctx context.Context, md pmetric.Metrics, _ string) error {
	batches, err := e.splitMetricsByRouting(md)
	if err != nil {
		return err
	}

	metricsByExporter := map[*wrappedExporter]pmetric.Metrics{}
	needsCleanup := true
	defer func() {
		if !needsCleanup {
			return
		}
		for exp := range metricsByExporter {
			exp.doneConsume()
		}
	}()

	routingIDs := make([]string, 0, len(batches))
	for routingID := range batches {
		routingIDs = append(routingIDs, routingID)
	}
	sort.Strings(routingIDs)

	for _, routingID := range routingIDs {
		mds := batches[routingID]
		exp, _, lookupErr := e.loadBalancer.exporterAndEndpoint([]byte(routingID))
		if lookupErr != nil {
			return consumererror.NewMetrics(lookupErr, mergeMetricsMapValues(batches))
		}

		expMetrics, ok := metricsByExporter[exp]
		if !ok {
			if !exp.tryStartConsume() {
				return consumererror.NewMetrics(errExporterIsStopping, mergeMetricsMapValues(batches))
			}
			expMetrics = pmetric.NewMetrics()
			metricsByExporter[exp] = expMetrics
		}

		metrics.Merge(expMetrics, mds)
	}

	failed := pmetric.NewMetrics()
	var errs error
	needsCleanup = false
	for exp, mds := range metricsByExporter {
		recordMetricBackendRequest(ctx, e.telemetry, exp.metricRequestAttr, mds)
		start := time.Now()
		err = exp.ConsumeMetrics(ctx, mds)
		duration := time.Since(start)

		exp.doneConsume()
		decision := e.recordBackendResultHealthOnly(ctx, exp, duration, err, true)
		if err != nil && decision.endpointLocal && !decision.failOpen && !endpointListContains(decision.eligible, exp.endpoint) {
			e.loadBalancer.cleanupBackendWithoutDrain(ctx, exp.endpoint)
		}
		if err == nil {
			continue
		}

		errs = errors.Join(errs, err)
		metrics.Merge(failed, mds)
	}

	if failed.DataPointCount() > 0 {
		return consumererror.NewMetrics(errs, failed)
	}

	return errs
}

func (e *metricExporterImp) recordBackendResult(ctx context.Context, we *wrappedExporter, duration time.Duration, err error, updateEndpointHealth bool) endpointHealthFailureDecision {
	return e.recordBackendResultWithDrain(ctx, we, duration, err, updateEndpointHealth, true)
}

func (e *metricExporterImp) recordBackendResultWithoutDrain(ctx context.Context, we *wrappedExporter, duration time.Duration, err error, updateEndpointHealth bool) endpointHealthFailureDecision {
	return e.recordBackendResultWithDrain(ctx, we, duration, err, updateEndpointHealth, false)
}

func (e *metricExporterImp) recordBackendResultWithDrain(ctx context.Context, we *wrappedExporter, duration time.Duration, err error, updateEndpointHealth, drainRemoved bool) endpointHealthFailureDecision {
	e.telemetry.LoadbalancerBackendLatency.Record(ctx, duration.Milliseconds(), metric.WithAttributeSet(we.endpointAttr))
	if err == nil {
		e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(we.successAttr))
		if updateEndpointHealth {
			e.loadBalancer.handleBackendSuccess(we.endpoint)
		}
		return endpointHealthFailureDecision{}
	}

	e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(we.failureAttr))
	e.logger.Debug("failed to export metrics", zap.Error(err))
	if !updateEndpointHealth {
		return endpointHealthFailureDecision{}
	}
	if drainRemoved {
		return e.loadBalancer.handleBackendFailure(ctx, we.endpoint, err)
	}
	return e.loadBalancer.handleBackendFailureWithoutDrain(ctx, we.endpoint, err)
}

func (e *metricExporterImp) recordBackendResultHealthOnly(ctx context.Context, we *wrappedExporter, duration time.Duration, err error, updateEndpointHealth bool) endpointHealthFailureDecision {
	e.telemetry.LoadbalancerBackendLatency.Record(ctx, duration.Milliseconds(), metric.WithAttributeSet(we.endpointAttr))
	if err == nil {
		e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(we.successAttr))
		if updateEndpointHealth {
			e.loadBalancer.handleBackendSuccess(we.endpoint)
		}
		return endpointHealthFailureDecision{}
	}

	e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(we.failureAttr))
	e.logger.Debug("failed to export metrics", zap.Error(err))
	if !updateEndpointHealth {
		return endpointHealthFailureDecision{}
	}
	return e.loadBalancer.handleBackendFailureHealthOnly(ctx, we.endpoint, err)
}

func wrapDirectMetricsRerouteError(err error, md pmetric.Metrics) error {
	if err == nil {
		return nil
	}
	var metricsErr consumererror.Metrics
	if errors.As(err, &metricsErr) {
		return err
	}
	return consumererror.NewMetrics(err, md)
}

func splitMetricsByResourceServiceName(md pmetric.Metrics) (map[string]pmetric.Metrics, []error) {
	results := map[string]pmetric.Metrics{}
	var errs []error

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		svc, ok := rm.Resource().Attributes().Get(string(conventions.ServiceNameKey))
		if !ok {
			errs = append(errs, fmt.Errorf("unable to get service name from resource metric with attributes: %v", rm.Resource().Attributes().AsRaw()))
			continue
		}

		newMD := pmetric.NewMetrics()
		rmClone := newMD.ResourceMetrics().AppendEmpty()
		rm.CopyTo(rmClone)

		key := svc.Str()
		existing, ok := results[key]
		if ok {
			metrics.Merge(existing, newMD)
		} else {
			results[key] = newMD
		}
	}

	return results, errs
}

func splitMetricsByResourceID(md pmetric.Metrics) map[string]pmetric.Metrics {
	results := map[string]pmetric.Metrics{}

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		newMD := pmetric.NewMetrics()
		rmClone := newMD.ResourceMetrics().AppendEmpty()
		rm.CopyTo(rmClone)

		key := identity.OfResource(rm.Resource()).String()
		existing, ok := results[key]
		if ok {
			metrics.Merge(existing, newMD)
		} else {
			results[key] = newMD
		}
	}

	return results
}

// splitMetricsByMetricName moves metric payloads out of md into per-name results.
// Callers must treat md as consumed after this returns.
func splitMetricsByMetricName(md pmetric.Metrics) map[string]pmetric.Metrics {
	results := map[string]pmetric.Metrics{}

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)

			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				key := m.Name()

				newMD, mClone := cloneMetricWithoutType(rm, sm, m)
				m.MoveTo(mClone)

				existing, ok := results[key]
				if ok {
					mergeMetricChunksByMove(existing, newMD)
				} else {
					results[key] = newMD
				}
			}
		}
	}

	return results
}

func splitMetricsByStreamID(md pmetric.Metrics) map[string]pmetric.Metrics {
	results := map[string]pmetric.Metrics{}

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		res := rm.Resource()

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scope := sm.Scope()

			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				metricID := identity.OfResourceMetric(res, scope, m)

				switch m.Type() {
				case pmetric.MetricTypeGauge:
					gauge := m.Gauge()

					for l := 0; l < gauge.DataPoints().Len(); l++ {
						dp := gauge.DataPoints().At(l)

						newMD, mClone := cloneMetricWithoutType(rm, sm, m)
						gaugeClone := mClone.SetEmptyGauge()

						dpClone := gaugeClone.DataPoints().AppendEmpty()
						dp.CopyTo(dpClone)

						key := identity.OfStream(metricID, dp).String()
						existing, ok := results[key]
						if ok {
							metrics.Merge(existing, newMD)
						} else {
							results[key] = newMD
						}
					}
				case pmetric.MetricTypeSum:
					sum := m.Sum()

					for l := 0; l < sum.DataPoints().Len(); l++ {
						dp := sum.DataPoints().At(l)

						newMD, mClone := cloneMetricWithoutType(rm, sm, m)
						sumClone := mClone.SetEmptySum()
						sumClone.SetIsMonotonic(sum.IsMonotonic())
						sumClone.SetAggregationTemporality(sum.AggregationTemporality())

						dpClone := sumClone.DataPoints().AppendEmpty()
						dp.CopyTo(dpClone)

						key := identity.OfStream(metricID, dp).String()
						existing, ok := results[key]
						if ok {
							metrics.Merge(existing, newMD)
						} else {
							results[key] = newMD
						}
					}
				case pmetric.MetricTypeHistogram:
					histogram := m.Histogram()

					for l := 0; l < histogram.DataPoints().Len(); l++ {
						dp := histogram.DataPoints().At(l)

						newMD, mClone := cloneMetricWithoutType(rm, sm, m)
						histogramClone := mClone.SetEmptyHistogram()
						histogramClone.SetAggregationTemporality(histogram.AggregationTemporality())

						dpClone := histogramClone.DataPoints().AppendEmpty()
						dp.CopyTo(dpClone)

						key := identity.OfStream(metricID, dp).String()
						existing, ok := results[key]
						if ok {
							metrics.Merge(existing, newMD)
						} else {
							results[key] = newMD
						}
					}
				case pmetric.MetricTypeExponentialHistogram:
					expHistogram := m.ExponentialHistogram()

					for l := 0; l < expHistogram.DataPoints().Len(); l++ {
						dp := expHistogram.DataPoints().At(l)

						newMD, mClone := cloneMetricWithoutType(rm, sm, m)
						expHistogramClone := mClone.SetEmptyExponentialHistogram()
						expHistogramClone.SetAggregationTemporality(expHistogram.AggregationTemporality())

						dpClone := expHistogramClone.DataPoints().AppendEmpty()
						dp.CopyTo(dpClone)

						key := identity.OfStream(metricID, dp).String()
						existing, ok := results[key]
						if ok {
							metrics.Merge(existing, newMD)
						} else {
							results[key] = newMD
						}
					}
				case pmetric.MetricTypeSummary:
					summary := m.Summary()

					for l := 0; l < summary.DataPoints().Len(); l++ {
						dp := summary.DataPoints().At(l)

						newMD, mClone := cloneMetricWithoutType(rm, sm, m)
						sumClone := mClone.SetEmptySummary()

						dpClone := sumClone.DataPoints().AppendEmpty()
						dp.CopyTo(dpClone)

						key := identity.OfStream(metricID, dp).String()
						existing, ok := results[key]
						if ok {
							metrics.Merge(existing, newMD)
						} else {
							results[key] = newMD
						}
					}
				}
			}
		}
	}

	return results
}

func cloneMetricWithoutType(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric) (md pmetric.Metrics, mClone pmetric.Metric) {
	md = pmetric.NewMetrics()

	rmClone := md.ResourceMetrics().AppendEmpty()
	rm.Resource().CopyTo(rmClone.Resource())
	rmClone.SetSchemaUrl(rm.SchemaUrl())

	smClone := rmClone.ScopeMetrics().AppendEmpty()
	sm.Scope().CopyTo(smClone.Scope())
	smClone.SetSchemaUrl(sm.SchemaUrl())

	mClone = smClone.Metrics().AppendEmpty()
	mClone.SetName(m.Name())
	mClone.SetDescription(m.Description())
	mClone.SetUnit(m.Unit())

	return md, mClone
}

func mergeMetricsMapValues(batches map[string]pmetric.Metrics) pmetric.Metrics {
	merged := pmetric.NewMetrics()
	for _, md := range batches {
		metrics.Merge(merged, md)
	}
	return merged
}
