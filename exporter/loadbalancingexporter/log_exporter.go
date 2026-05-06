// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
)

var _ exporter.Logs = (*logExporterImp)(nil)

type logExporterImp struct {
	loadBalancer *loadBalancer
	batcher      *logBatcher
	centralQueue *centralQueue
	centralCodec *queuePayloadCodec

	logger        *zap.Logger
	started       atomic.Bool
	telemetry     *metadata.TelemetryBuilder
	ignoreTraceID bool
	randomTraceID func() pcommon.TraceID
	centralCancel context.CancelFunc
	centralWG     sync.WaitGroup
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

	logExporter := &logExporterImp{
		loadBalancer:  lb,
		telemetry:     telemetry,
		logger:        params.Logger,
		ignoreTraceID: cfg.(*Config).LogRouting.IgnoreTraceID,
		randomTraceID: random,
	}
	if cfg.(*Config).CentralQueue.Enabled {
		centralCfg := cfg.(*Config).CentralQueue
		centralTelemetry, telemetryErr := newCentralQueueTelemetry(params.TelemetrySettings, signalKindLogs)
		if telemetryErr != nil {
			return nil, telemetryErr
		}
		logExporter.centralQueue = newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           centralCfg.MaxCompressedBytes,
			maxInflightUncompressedBytes: centralCfg.MaxInflightUncompressedBytes,
			maxUncompressedBatchBytes:    centralCfg.MaxUncompressedBatchBytes,
			telemetry:                    centralTelemetry,
		})
		logExporter.centralCodec = newQueuePayloadCodec(centralCfg.PayloadCompression)
	}
	if cfg.(*Config).LogBatcher.Enabled {
		logBatcherCfg := cfg.(*Config).LogBatcher
		logExporter.batcher, err = newLogBatcher(
			params.Logger,
			params.TelemetrySettings,
			logBatcherSettings{
				maxRecords:         logBatcherCfg.MaxRecords,
				maxBytes:           logBatcherCfg.MaxBytes,
				flushInterval:      logBatcherCfg.FlushInterval,
				payloadCompression: logBatcherCfg.PayloadCompression,
			},
			logExporter.consumeBatcherFlush,
			logExporter.rerouteDrainBatch,
		)
		if err != nil {
			return nil, err
		}
		lb.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
			return logExporter.batcher.Remove(ctx, endpoint, exp)
		}
	}

	return logExporter, nil
}

func (*logExporterImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *logExporterImp) Start(ctx context.Context, host component.Host) error {
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

func (e *logExporterImp) Shutdown(ctx context.Context) error {
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

func (e *logExporterImp) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if !e.started.Load() {
		return errExporterIsStopping
	}
	if e.centralQueue != nil {
		return e.consumeLogsCentralQueue(ctx, ld)
	}
	if e.batcher == nil {
		var errs error
		batches := batchpersignal.SplitLogs(ld)
		for _, batch := range batches {
			errs = multierr.Append(errs, e.consumeLog(ctx, batch))
		}
		return errs
	}

	return e.consumeLogsBatched(ctx, ld)
}

func (e *logExporterImp) consumeLogsCentralQueue(_ context.Context, ld plog.Logs) error {
	batches := e.groupLogsByRoutingKey(ld)
	var errs error
	now := time.Now()
	for routingKey, logs := range batches {
		item, err := newCentralQueueLogsItem(routingKey[:], logs, e.centralCodec, now)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		errs = multierr.Append(errs, e.centralQueue.enqueue(item))
	}
	return errs
}

func (e *logExporterImp) groupLogsByRoutingKey(ld plog.Logs) map[pcommon.TraceID]plog.Logs {
	batches := make(map[pcommon.TraceID]plog.Logs)
	emptyTraceFallbackKeys := make(map[[2]int]pcommon.TraceID)

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				rec := sl.LogRecords().At(k)
				balancingKey := e.routingKeyForLogRecord(rec, [2]int{i, j}, emptyTraceFallbackKeys)
				batch, ok := batches[balancingKey]
				if !ok {
					batch = plog.NewLogs()
					batches[balancingKey] = batch
				}
				insertLogRecord(batch, rl, sl, rec)
			}
		}
	}

	return batches
}

func (e *logExporterImp) runCentralQueue(ctx context.Context) {
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
			e.logger.Warn("failed to lease central log queue item", zap.Error(err))
			continue
		}

		err = e.consumeCentralQueueLogItem(ctx, lease.item)
		if err == nil {
			lease.done()
			continue
		}
		if ctx.Err() != nil {
			lease.done()
			return
		}
		if consumererror.IsPermanent(err) {
			e.logger.Warn("dropping central log queue item after permanent export error", zap.Error(err))
			lease.done()
			continue
		}
		item := lease.item
		nextAttempt := time.Now().Add(centralQueueRetryDelay(item.attempt))
		item.attempt++
		lease.item = item
		if requeueErr := lease.requeue(nextAttempt); requeueErr != nil {
			e.logger.Warn("failed to requeue central log queue item", zap.Error(requeueErr), zap.Error(err))
		}
	}
}

func (e *logExporterImp) consumeCentralQueueLogItem(ctx context.Context, item centralQueueItem) error {
	ld, err := decodeCentralQueueLogsItem(item, e.centralCodec)
	if err != nil {
		e.logger.Warn("dropping invalid central log queue payload", zap.Error(err))
		return nil
	}
	le, _, err := e.loadBalancer.exporterAndEndpoint(item.routingKey)
	if err != nil {
		return err
	}
	_, err = e.consumeBatchWithDecision(ctx, le, ld, logFlushReasonDirect, true, false, false)
	return err
}

// endpointBatch holds logs grouped for a single backend endpoint,
// with resource/scope structures properly deduplicated.
type endpointBatch struct {
	logs plog.Logs
	exp  *wrappedExporter
}

// consumeLogsBatched routes each log record to its target endpoint via the
// load balancer, grouping records into per-endpoint plog.Logs that preserve
// the original resource/scope hierarchy. This avoids the N×resource/scope
// duplication that per-record wrapping would cause in the batcher.
func (e *logExporterImp) consumeLogsBatched(ctx context.Context, ld plog.Logs) error {
	batches, errs := e.groupLogsByEndpoint(ld)
	return multierr.Append(errs, e.enqueueEndpointBatches(ctx, batches, true))
}

func (e *logExporterImp) groupLogsByEndpoint(ld plog.Logs) (map[string]*endpointBatch, error) {
	batches := make(map[string]*endpointBatch)
	emptyTraceFallbackKeys := make(map[[2]int]pcommon.TraceID)
	var errs error

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				rec := sl.LogRecords().At(k)

				balancingKey := e.routingKeyForLogRecord(rec, [2]int{i, j}, emptyTraceFallbackKeys)

				le, endpoint, err := e.loadBalancer.exporterAndEndpoint(balancingKey[:])
				if err != nil {
					errs = multierr.Append(errs, err)
					continue
				}
				ep := endpointWithPort(endpoint)
				batch, ok := batches[ep]
				if !ok {
					batch = &endpointBatch{logs: plog.NewLogs(), exp: le}
					batches[ep] = batch
				}
				batch.exp = le
				insertLogRecord(batch.logs, rl, sl, rec)
			}
		}
	}

	return batches, errs
}

func (e *logExporterImp) enqueueEndpointBatches(ctx context.Context, batches map[string]*endpointBatch, retryOnStopping bool) error {
	var errs error

	for ep, batch := range batches {
		err := e.batcher.Enqueue(ctx, ep, batch.exp, batch.logs)
		if errors.Is(err, errLogBatcherExporterStopping) && retryOnStopping {
			reroutedBatches, rerouteErr := e.groupLogsByEndpoint(batch.logs)
			errs = multierr.Append(errs, rerouteErr)
			errs = multierr.Append(errs, e.enqueueEndpointBatches(ctx, reroutedBatches, false))
			continue
		}
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	return errs
}

func (e *logExporterImp) consumeLog(ctx context.Context, ld plog.Logs) error {
	return e.consumeLogDirect(ctx, ld, 0)
}

func (e *logExporterImp) consumeLogDirect(ctx context.Context, ld plog.Logs, rerouteAttempt int) error {
	balancingKey := e.routingKeyForLogs(ld)

	le, _, err := e.loadBalancer.exporterAndEndpoint(balancingKey[:])
	if err != nil {
		return err
	}

	var retryLogs plog.Logs
	if directRerouteAttemptAllowed(e.loadBalancer, rerouteAttempt) {
		retryLogs = plog.NewLogs()
		ld.CopyTo(retryLogs)
	}
	decision, err := e.consumeBatchWithDecision(ctx, le, ld, logFlushReasonDirect, true, true, false)
	if err != nil && shouldRerouteDirectFailure(e.loadBalancer, le.endpoint, decision, rerouteAttempt) {
		rerouteErr := e.consumeLogDirect(ctx, retryLogs, rerouteAttempt+1)
		e.loadBalancer.recordBackendReroute(ctx, "logs", decision.reason, rerouteErr)
		return rerouteErr
	}
	return err
}

func (e *logExporterImp) consumeBatch(ctx context.Context, le *wrappedExporter, ld plog.Logs, reason string) error {
	_, err := e.consumeBatchWithDecision(ctx, le, ld, reason, false, true, false)
	return err
}

func (e *logExporterImp) consumeBatcherFlush(ctx context.Context, le *wrappedExporter, ld plog.Logs, reason string) error {
	decision, err := e.consumeBatchWithDecision(ctx, le, ld, reason, reason != logFlushReasonShutdown, false, true)
	if errors.Is(err, errLogBatcherExporterStopping) && reason != logFlushReasonShutdown && directRerouteAttemptAllowed(e.loadBalancer, 0) {
		return logBatcherRerouteableError{
			err: err,
			recordReroute: func(ctx context.Context, rerouteErr error) {
				e.loadBalancer.recordBackendReroute(ctx, "logs", endpointFailureExporterStopping, rerouteErr)
			},
		}
	}
	if err != nil && shouldRerouteDirectFailure(e.loadBalancer, le.endpoint, decision, 0) {
		e.loadBalancer.cleanupBackendWithoutDrain(ctx, le.endpoint)
		return logBatcherRerouteableError{
			err: err,
			recordReroute: func(ctx context.Context, rerouteErr error) {
				e.loadBalancer.recordBackendReroute(ctx, "logs", decision.reason, rerouteErr)
			},
		}
	}
	return err
}

type logBatcherRerouteableError struct {
	err           error
	recordReroute func(context.Context, error)
}

func (e logBatcherRerouteableError) Error() string {
	return e.err.Error()
}

func (e logBatcherRerouteableError) Unwrap() error {
	return e.err
}

func (e logBatcherRerouteableError) RecordReroute(ctx context.Context, err error) {
	if e.recordReroute != nil {
		e.recordReroute(ctx, err)
	}
}

func (e *logExporterImp) rerouteDrainBatch(ctx context.Context, ld plog.Logs, reason string) error {
	if reason == logFlushReasonShutdown || !directRerouteAttemptAllowed(e.loadBalancer, 0) {
		return errLogBatcherExporterStopping
	}
	batches, errs := e.groupLogsByEndpoint(ld)
	for _, batch := range batches {
		decision, err := e.consumeBatchWithDecision(ctx, batch.exp, batch.logs, reason, true, false, true)
		if err != nil && decision.endpointLocal && !decision.failOpen && !endpointListContains(decision.eligible, batch.exp.endpoint) {
			e.loadBalancer.cleanupBackendWithoutDrain(ctx, batch.exp.endpoint)
		}
		errs = multierr.Append(errs, err)
	}
	return errs
}

func (e *logExporterImp) consumeBatchWithDecision(ctx context.Context, le *wrappedExporter, ld plog.Logs, reason string, updateEndpointHealth, drainRemoved, healthOnly bool) (endpointHealthFailureDecision, error) {
	if reason == logFlushReasonDirect || reason == logFlushReasonResolverChange || reason == logFlushReasonShutdown {
		le.forceStartConsume()
	} else if !le.tryStartConsume() {
		return endpointHealthFailureDecision{}, errLogBatcherExporterStopping
	}
	defer le.doneConsume()

	start := time.Now()
	err := le.ConsumeLogs(ctx, ld)
	duration := time.Since(start)
	e.telemetry.LoadbalancerBackendLatency.Record(ctx, duration.Milliseconds(), metric.WithAttributeSet(le.endpointAttr))
	if err == nil {
		e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(le.successAttr))
		if updateEndpointHealth {
			e.loadBalancer.handleBackendSuccess(le.endpoint)
		}
		return endpointHealthFailureDecision{}, nil
	}

	e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(le.failureAttr))
	e.logger.Debug("failed to export log", zap.Error(err))
	if !updateEndpointHealth {
		return endpointHealthFailureDecision{}, err
	}
	if healthOnly {
		return e.loadBalancer.handleBackendFailureHealthOnly(ctx, le.endpoint, err), err
	}
	if drainRemoved {
		return e.loadBalancer.handleBackendFailure(ctx, le.endpoint, err), err
	}
	return e.loadBalancer.handleBackendFailureWithoutDrain(ctx, le.endpoint, err), err
}

// insertLogRecord adds a log record into the destination plog.Logs, reusing
// existing ResourceLogs/ScopeLogs entries when their attributes match. This
// avoids creating duplicate resource/scope hierarchies for records that share
// the same origin.
func insertLogRecord(dest plog.Logs, srcRL plog.ResourceLogs, srcSL plog.ScopeLogs, rec plog.LogRecord) {
	targetRL := findOrCreateResourceLogs(dest, srcRL)
	targetSL := findOrCreateScopeLogs(targetRL, srcSL)
	rec.CopyTo(targetSL.LogRecords().AppendEmpty())
}

func findOrCreateResourceLogs(dest plog.Logs, src plog.ResourceLogs) plog.ResourceLogs {
	srcRes := src.Resource()
	for i := 0; i < dest.ResourceLogs().Len(); i++ {
		rl := dest.ResourceLogs().At(i)
		if resourceLogsMatches(rl, src) {
			return rl
		}
	}
	rl := dest.ResourceLogs().AppendEmpty()
	srcRes.CopyTo(rl.Resource())
	rl.SetSchemaUrl(src.SchemaUrl())
	return rl
}

func findOrCreateScopeLogs(rl plog.ResourceLogs, src plog.ScopeLogs) plog.ScopeLogs {
	srcScope := src.Scope()
	for i := 0; i < rl.ScopeLogs().Len(); i++ {
		sl := rl.ScopeLogs().At(i)
		if scopeLogsMatches(sl, src) {
			return sl
		}
	}
	sl := rl.ScopeLogs().AppendEmpty()
	srcScope.CopyTo(sl.Scope())
	sl.SetSchemaUrl(src.SchemaUrl())
	return sl
}

func (e *logExporterImp) routingKeyForLogRecord(rec plog.LogRecord, scopeKey [2]int, emptyTraceFallbackKeys map[[2]int]pcommon.TraceID) pcommon.TraceID {
	if !e.ignoreTraceID {
		traceID := rec.TraceID()
		if traceID != pcommon.NewTraceIDEmpty() {
			return traceID
		}
	}

	balancingKey := emptyTraceFallbackKeys[scopeKey]
	if balancingKey == pcommon.NewTraceIDEmpty() {
		balancingKey = e.nextRandomTraceID()
		emptyTraceFallbackKeys[scopeKey] = balancingKey
	}
	return balancingKey
}

func (e *logExporterImp) routingKeyForLogs(ld plog.Logs) pcommon.TraceID {
	if !e.ignoreTraceID {
		traceID := traceIDFromLogs(ld)
		if traceID != pcommon.NewTraceIDEmpty() {
			return traceID
		}
	}
	return e.nextRandomTraceID()
}

func (e *logExporterImp) nextRandomTraceID() pcommon.TraceID {
	if e.randomTraceID == nil {
		return random()
	}
	return e.randomTraceID()
}

func traceIDFromLogs(ld plog.Logs) pcommon.TraceID {
	rl := ld.ResourceLogs()
	if rl.Len() == 0 {
		return pcommon.NewTraceIDEmpty()
	}

	sl := rl.At(0).ScopeLogs()
	if sl.Len() == 0 {
		return pcommon.NewTraceIDEmpty()
	}

	logs := sl.At(0).LogRecords()
	if logs.Len() == 0 {
		return pcommon.NewTraceIDEmpty()
	}

	return logs.At(0).TraceID()
}

func random() pcommon.TraceID {
	v1 := uint8(rand.IntN(256))
	v2 := uint8(rand.IntN(256))
	v3 := uint8(rand.IntN(256))
	v4 := uint8(rand.IntN(256))
	return [16]byte{v1, v2, v3, v4}
}
