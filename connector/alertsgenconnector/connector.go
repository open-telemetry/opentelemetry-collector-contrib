// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/state"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/telemetry"
)

// alertsConnector evaluates streaming telemetry against rules and forwards the
// original data to the next consumer (Traces->Traces, Logs->Logs, Metrics->Metrics).
type alertsConnector struct {
	cfg    *Config
	logger *zap.Logger
	rs     *ruleSet
	ing    *ingester
	mx     *telemetry.Metrics
	tsdb   *state.TSDBSyncer // optional; nil if HA/TSDB is not configured

	// downstreams (only one of these will be set depending on factory used)
	nextTraces  consumer.Traces
	nextLogs    consumer.Logs
	nextMetrics consumer.Metrics

	// Batching for remote write
	eventBatch   []state.AlertEvent
	eventBatchMu sync.Mutex
	batchTicker  *time.Ticker
	flushChan    chan struct{}

	evalMu   sync.Mutex
	evalStop chan struct{}
	wg       sync.WaitGroup
}

func newAlertsConnector(ctx context.Context, set connector.Settings, cfg component.Config) (*alertsConnector, error) {
	c, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type %T", cfg)
	}

	// compile rules
	rs, err := compileRules(c)
	if err != nil {
		return nil, err
	}

	// self telemetry
	mx, err := telemetry.New(set.MeterProvider)
	if err != nil {
		set.Logger.Warn("Failed to create telemetry", zap.Error(err))
	}
	rs.mx = mx

	// ingester holds sliding windows / buffers with adaptive memory management
	ing := newIngesterWithLogger(c, set.Logger)

	// OPTIONAL TSDB (only when enabled and URL set)
	var ts *state.TSDBSyncer
	if c.TSDB != nil && c.TSDB.Enabled && c.TSDB.QueryURL != "" {
		tsdbCfg := state.TSDBConfig{
			QueryURL:       c.TSDB.QueryURL,
			RemoteWriteURL: c.TSDB.RemoteWriteURL,
			QueryTimeout:   c.TSDB.QueryTimeout,
			WriteTimeout:   c.TSDB.WriteTimeout,
			DedupWindow:    c.TSDB.DedupWindow,
			InstanceID:     c.InstanceID,
		}
		ts, err = state.NewTSDBSyncer(tsdbCfg)
		if err != nil {
			set.Logger.Warn("Failed to create TSDB syncer", zap.Error(err))
		} else {
			// Restore state from TSDB on startup
			if err := rs.restoreFromTSDB(ts); err != nil {
				set.Logger.Warn("Failed to restore state from TSDB", zap.Error(err))
			} else {
				set.Logger.Info("Successfully restored alert state from TSDB")
			}
		}
	} else {
		set.Logger.Info("TSDB disabled or not configured; using in-memory state")
	}

	ac := &alertsConnector{
		cfg:        c,
		logger:     set.Logger,
		rs:         rs,
		ing:        ing,
		mx:         mx,
		tsdb:       ts,
		eventBatch: make([]state.AlertEvent, 0, getBatchSize(c)),
		flushChan:  make(chan struct{}, 1),
		evalStop:   make(chan struct{}),
	}

	// remote write batching only when TSDB active AND enabled
	if ts != nil && c.TSDB.EnableRemoteWrite {
		ac.batchTicker = time.NewTicker(c.TSDB.RemoteWriteFlushInterval)
	}

	return ac, nil
}

func getBatchSize(c *Config) int {
	if c.TSDB != nil && c.TSDB.RemoteWriteBatchSize > 0 {
		return c.TSDB.RemoteWriteBatchSize
	}
	return 1000 // default
}

// ---- lifecycle --------------------------------------------------------------

func (e *alertsConnector) Start(ctx context.Context, _ component.Host) error {
	e.logger.Info("Starting alerts connector",
		zap.String("instance_id", e.cfg.InstanceID),
		zap.Duration("window_size", e.cfg.WindowSize),
		zap.Int("num_rules", len(e.cfg.Rules)),
		zap.Bool("adaptive_scaling_enabled", e.cfg.Memory.EnableAdaptiveScaling),
		zap.Bool("memory_pressure_handling_enabled", e.cfg.Memory.EnableMemoryPressureHandling),
		zap.Bool("use_ring_buffers", e.cfg.Memory.UseRingBuffers),
	)

	interval := e.cfg.Step
	if interval <= 0 {
		interval = e.cfg.WindowSize
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}

	// evaluation loop
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				e.evaluateOnce(time.Now())
			case <-e.evalStop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// batch flusher (only when enabled)
	if e.batchTicker != nil {
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			defer e.batchTicker.Stop()
			for {
				select {
				case <-e.batchTicker.C:
					e.flushEventBatch()
				case <-e.flushChan:
					e.flushEventBatch()
				case <-e.evalStop:
					e.flushEventBatch()
					return
				case <-ctx.Done():
					e.flushEventBatch()
					return
				}
			}
		}()
	}

	// memory reporting
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				e.reportMemoryUsage()
			case <-e.evalStop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (e *alertsConnector) Shutdown(ctx context.Context) error {
	e.logger.Info("Shutting down alerts connector")
	close(e.evalStop)

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		e.logger.Info("Alerts connector shutdown complete")
	case <-ctx.Done():
		e.logger.Warn("Alerts connector shutdown timed out")
	}

	e.reportMemoryUsage()
	return nil
}

func (e *alertsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// reportMemoryUsage logs detailed memory usage statistics
func (e *alertsConnector) reportMemoryUsage() {
	if e.ing == nil || e.ing.memMgr == nil {
		return
	}

	current, max, percent := e.ing.memMgr.GetMemoryUsage()
	stats := e.ing.memMgr.GetStats()
	traceLimit, logLimit, metricLimit := e.ing.memMgr.GetCurrentLimits()

	e.logger.Info("Connector memory usage report",
		zap.String("instance_id", e.cfg.InstanceID),
		zap.Int64("memory_current_bytes", current),
		zap.Int64("memory_max_bytes", max),
		zap.Float64("memory_usage_percent", percent),
		zap.Int("traces_buffered", e.ing.traces.Len()),
		zap.Int("logs_buffered", e.ing.logs.Len()),
		zap.Int("metrics_buffered", e.ing.metrics.Len()),
		zap.Int64("trace_limit", traceLimit),
		zap.Int64("log_limit", logLimit),
		zap.Int64("metric_limit", metricLimit),
		zap.Int64("total_dropped_traces", stats.DroppedTraces),
		zap.Int64("total_dropped_logs", stats.DroppedLogs),
		zap.Int64("total_dropped_metrics", stats.DroppedMetrics),
		zap.Int64("scale_up_events", stats.ScaleUpEvents),
		zap.Int64("scale_down_events", stats.ScaleDownEvents),
		zap.Int64("memory_pressure_events", stats.MemoryPressureEvents),
	)

	// Also emit telemetry metrics if available
	if e.mx != nil {
		ctx := context.Background()
		e.mx.RecordMemoryUsage(ctx, float64(current), percent)
		e.mx.RecordBufferSizes(ctx, e.ing.traces.Len(), e.ing.logs.Len(), e.ing.metrics.Len())
		e.mx.RecordDroppedData(ctx, stats.DroppedTraces, stats.DroppedLogs, stats.DroppedMetrics)
	}
}

// ---- evaluation -------------------------------------------------------------

func (e *alertsConnector) evaluateOnce(now time.Time) {
	e.evalMu.Lock()
	defer e.evalMu.Unlock()

	start := time.Now()
	events, metrics := e.rs.evaluate(now, e.ing)
	evalDuration := time.Since(start)

	if len(events) > 0 {
		e.logger.Debug("Generated alert events",
			zap.Int("count", len(events)),
			zap.Duration("eval_duration", evalDuration),
		)

		// Convert to TSDB events and add to batch
		tsdbEvents := e.convertToTSDBEvents(events, now)
		e.addEventsToBatch(tsdbEvents)

		// NEW: Emit a metric per alert event (when enabled & metrics pipeline exists)
		if e.cfg.EmitAlertMetrics && e.nextMetrics != nil {
			md := e.buildAlertMetrics(events, now)
			if md.MetricCount() > 0 {
				if err := e.nextMetrics.ConsumeMetrics(context.Background(), md); err != nil {
					e.logger.Warn("Failed to emit alert metrics", zap.Error(err))
				}
			}
		}

		// Self-telemetry
		if e.mx != nil {
			e.mx.RecordEvents(context.Background(), len(events))
		}
	}

	// Update active alert metrics
	if e.mx != nil && len(metrics) > 0 {
		for _, metric := range metrics {
			if metric.Active > 0 {
				e.mx.AddActive(context.Background(), 1)
			}
		}
	}

	// Record evaluation metrics
	if e.mx != nil {
		e.mx.RecordEvaluation(context.Background(), evalDuration)
	}

	// Log slow evaluations
	if evalDuration > 5*time.Second {
		e.logger.Warn("Slow alert evaluation detected",
			zap.Duration("duration", evalDuration),
			zap.Int("events_generated", len(events)),
			zap.Int("metrics_generated", len(metrics)),
		)
	}
}

// buildAlertMetrics converts alert events into a pmetric.Metrics payload.
// It creates a counter "alertsgen_alerts_total" with one data point per event.
func (e *alertsConnector) buildAlertMetrics(events []alertEvent, ts time.Time) pmetric.Metrics {
	md := pmetric.NewMetrics()
	if len(events) == 0 {
		return md
	}

	rm := md.ResourceMetrics().AppendEmpty()
	ilm := rm.ScopeMetrics().AppendEmpty()
	sm := ilm.Metrics().AppendEmpty()
	sm.SetName("alertsgen_alerts_total")
	sm.SetEmptySum()
	sum := sm.Sum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	dps := sum.DataPoints()
	for _, ev := range events {
		dp := dps.AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		dp.SetIntValue(1)

		attrs := dp.Attributes()

		// Core attributes mirroring the alert log event.
		// Note: ev.Labels may already contain "alertname".
		if an, ok := ev.Labels["alertname"]; ok && an != "" {
			attrs.PutStr("alertname", an)
		}
		if ev.Rule != "" {
			attrs.PutStr("rule_id", ev.Rule)
		}
		if ev.Severity != "" {
			attrs.PutStr("severity", ev.Severity)
		}
		if ev.State != "" {
			attrs.PutStr("state", string(ev.State))
		}
		if fmt.Sprint(ev.Window) != "" && fmt.Sprint(ev.Window) != "0" {
			attrs.PutStr("window", fmt.Sprint(ev.Window))
		}
		if fmt.Sprint(ev.For) != "" && fmt.Sprint(ev.For) != "0" {
			attrs.PutStr("for", fmt.Sprint(ev.For))
		}

		// Flatten all labels; skip alertname if we already set it above.
		for k, v := range ev.Labels {
			if k == "alertname" {
				continue
			}
			attrs.PutStr(k, v)
		}
	}

	return md
}

// convertToTSDBEvents converts internal alertEvent to state.AlertEvent
func (e *alertsConnector) convertToTSDBEvents(events []alertEvent, timestamp time.Time) []state.AlertEvent {
	tsdbEvents := make([]state.AlertEvent, 0, len(events))

	for _, event := range events {
		// Calculate fingerprint
		fp := fingerprint(event.Rule, event.Labels)

		tsdbEvent := state.AlertEvent{
			Rule:        event.Rule,
			State:       event.State,
			Severity:    event.Severity,
			Labels:      event.Labels,
			Value:       event.Value,
			Window:      event.Window,
			For:         event.For,
			Timestamp:   timestamp,
			Fingerprint: fp,
		}

		tsdbEvents = append(tsdbEvents, tsdbEvent)
	}

	return tsdbEvents
}

// addEventsToBatch adds events to the batch and triggers flush if needed
func (e *alertsConnector) addEventsToBatch(events []state.AlertEvent) {
	if e.tsdb == nil || e.cfg.TSDB == nil || !e.cfg.TSDB.EnableRemoteWrite {
		return
	}
	e.eventBatchMu.Lock()
	e.eventBatch = append(e.eventBatch, events...)
	shouldFlush := len(e.eventBatch) >= e.cfg.TSDB.RemoteWriteBatchSize
	e.eventBatchMu.Unlock()

	if shouldFlush {
		select {
		case e.flushChan <- struct{}{}:
		default:
		}
	}
}

// flushEventBatch sends accumulated events to TSDB
func (e *alertsConnector) flushEventBatch() {
	if e.tsdb == nil || e.cfg.TSDB == nil || !e.cfg.TSDB.EnableRemoteWrite {
		return
	}

	e.eventBatchMu.Lock()
	if len(e.eventBatch) == 0 {
		e.eventBatchMu.Unlock()
		return
	}
	eventsToFlush := make([]state.AlertEvent, len(e.eventBatch))
	copy(eventsToFlush, e.eventBatch)
	e.eventBatch = e.eventBatch[:0]
	e.eventBatchMu.Unlock()

	interfaceEvents := make([]interface{}, len(eventsToFlush))
	for i, ev := range eventsToFlush {
		interfaceEvents[i] = ev
	}

	start := time.Now()
	if err := e.tsdb.PublishEvents(interfaceEvents); err != nil {
		e.logger.Warn("Failed to publish events to TSDB",
			zap.Error(err),
			zap.Int("event_count", len(eventsToFlush)),
			zap.Duration("duration", time.Since(start)),
		)
		if e.mx != nil {
			e.mx.RecordDroppedData(context.Background(), int64(len(eventsToFlush)), 0, 0)
			e.mx.RecordDropped(context.Background(), "tsdb_publish_failed")
		}
	} else {
		e.logger.Debug("Successfully published events to TSDB",
			zap.Int("event_count", len(eventsToFlush)),
			zap.Duration("duration", time.Since(start)),
		)
	}
}

// ---- Consume methods ---------------------------------------

func (e *alertsConnector) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// Ingest into sliding window
	if err := e.ing.consumeTraces(td); err != nil {
		e.logger.Error("Failed to ingest traces", zap.Error(err))
		return err
	}
	// Forward downstream unchanged
	if e.nextTraces != nil {
		return e.nextTraces.ConsumeTraces(ctx, td)
	}
	return nil
}

func (e *alertsConnector) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if err := e.ing.consumeLogs(ld); err != nil {
		e.logger.Error("Failed to ingest logs", zap.Error(err))
		return err
	}
	if e.nextLogs != nil {
		return e.nextLogs.ConsumeLogs(ctx, ld)
	}
	return nil
}

func (e *alertsConnector) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if err := e.ing.consumeMetrics(md); err != nil {
		e.logger.Error("Failed to ingest metrics", zap.Error(err))
		return err
	}
	if e.nextMetrics != nil {
		return e.nextMetrics.ConsumeMetrics(ctx, md)
	}
	return nil
}
