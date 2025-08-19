
package alertsgenconnector

import (
	"context"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/cardinality"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/dedup"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/notify"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/state"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/storm"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/telemetry"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const typeStr = "alertsgen"

// Factory
func NewFactory() connector.Factory {
	return connector.NewFactory(
		component.Type(typeStr),
		createDefaultConfig,
		connector.WithTracesToLogs(createTracesToLogs, component.StabilityLevelDevelopment),
		connector.WithTracesToMetrics(createTracesToMetrics, component.StabilityLevelDevelopment),
		connector.WithLogsToLogs(createLogsToLogs, component.StabilityLevelDevelopment),
		connector.WithLogsToMetrics(createLogsToMetrics, component.StabilityLevelDevelopment),
		connector.WithMetricsToMetrics(createMetricsToMetrics, component.StabilityLevelDevelopment),
		connector.WithMetricsToLogs(createMetricsToLogs, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		WindowSize: 5 * time.Second,
		Storm: StormConfig{
			MaxAlertsPerMinute:      100,
			CircuitBreakerThreshold: 0.5,
		},
		Cardinality: CardinalityConfig{
			MaxLabelsPerAlert: 20,
			MaxSeriesPerRule:  1000,
			HashIfExceeds:     128,
		},
		Notify: NotifyConfig{
			Timeout: 5 * time.Second,
		},
	}
}

// Engine holds shared state for all connector edges created for the same ID.
type Engine struct {
	cfg *Config
	set connector.CreateSettings

	ing *ingester
	rs  *ruleSet

	limiter *storm.Limiter
	card    *cardinality.Limiter
	deduper *dedup.Deduper
	syncer  *state.TSDBSyncer

	metrics *telemetry.Metrics

	// downstreams
	tracesOut  consumer.Traces
	logsOut    consumer.Logs
	metricsOut consumer.Metrics

	tickCh chan struct{}
	stopCh chan struct{}
}

type engineHandle struct{ ref int; eng *Engine }

var engines = map[component.ID]*engineHandle{}

func getOrCreateEngine(cfg *Config, set connector.CreateSettings) (*Engine, func(), error) {
	if h, ok := engines=set.ID; ok {
		h.ref++
		return h.eng, func(){ releaseEngine(set.ID) }, nil
	}
	eng, err := NewEngine(cfg, set)
	if err != nil { return nil, nil, err }
	engines[set.ID] = &engineHandle{ref:1, eng:eng}
	return eng, func(){ releaseEngine(set.ID) }, nil
}

func releaseEngine(id component.ID) {
	if h, ok := engines[id]; ok {
		h.ref--
		if h.ref == 0 {
			h.eng.Close()
			delete(engines, id)
		}
	}
}

func NewEngine(cfg *Config, set connector.CreateSettings) (*Engine, error) {
	if err := cfg.Validate(); err != nil { return nil, err }
	rs, err := compileRules(cfg)
	if err != nil { return nil, err }

	var mx *telemetry.Metrics
	if set.TelemetrySettings.MeterProvider != nil {
		mx, _ = telemetry.New(set.TelemetrySettings.MeterProvider)
	}

	e := &Engine{
		cfg:     cfg,
		set:     set,
		ing:     newIngester(),
		rs:      rs,
		limiter: storm.NewLimiter(cfg.Storm.MaxAlertsPerMinute),
		card:    cardinality.NewLimiter(cfg.Cardinality.MaxLabelsPerAlert, cfg.Cardinality.HashIfExceeds),
		deduper: dedup.NewDeduper(cfg.Dedup.Window, cfg.Dedup.FingerprintLabels, cfg.Dedup.ExcludeLabels),
		tickCh:  make(chan struct{}, 1),
		stopCh:  make(chan struct{}),
		metrics: mx,
	}
	if cfg.TSDB.QueryURL != "" {
		e.syncer = &state.TSDBSyncer{
			QueryURL:        cfg.TSDB.QueryURL,
			InstanceID:      cfg.InstanceID,
			QueryInterval:   cfg.TSDB.QueryInterval,
			TakeoverTimeout: cfg.TSDB.TakeoverTimeout,
		}
	}
	// wire telemetry into ruleset
	rs.mx = mx
	return e, nil
}

func (e *Engine) SetTracesConsumer(next consumer.Traces)  { e.tracesOut = next }
func (e *Engine) SetLogsConsumer(next consumer.Logs)      { e.logsOut = next }
func (e *Engine) SetMetricsConsumer(next consumer.Metrics){ e.metricsOut = next }

func (e *Engine) Start(ctx context.Context, _ component.Host) error {
	go func() {
		if e.syncer != nil {
			_ = e.rs.restoreFromTSDB(e.syncer)
		}
	}()
	go e.loop()
	return nil
}
func (e *Engine) Close() { _ = e.Shutdown(context.Background()) }
func (e *Engine) Shutdown(ctx context.Context) error { close(e.stopCh); return nil }

func (e *Engine) loop() {
	ticker := time.NewTicker(e.cfg.WindowSize)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.evaluateOnce()
		case <-e.tickCh:
			e.evaluateOnce()
		case <-e.stopCh:
			return
		}
	}
}

func (e *Engine) evaluateOnce() {
	ctx := context.Background()
	start := time.Now()

	events, metricsView := e.rs.evaluate(start, e.ing)

	var filtered []alertEvent
	droppedLimiter := 0
	droppedDedup := 0

	for _, ev := range events {
		if !e.limiter.Allow() {
			droppedLimiter++
			continue
		}
		ev.Labels = e.card.Enforce(ev.Labels)
		if !e.deduper.Allow(ev.Rule, ev.Labels) {
			droppedDedup++
			continue
		}
		filtered = append(filtered, ev)
	}

	if e.metrics != nil {
		e.metrics.RecordEvaluation(ctx, "all_rules", "success", time.Since(start))
		if droppedLimiter > 0 { e.metrics.RecordDropped(ctx, droppedLimiter, "storm_limiter") }
		if droppedDedup > 0 { e.metrics.RecordDropped(ctx, droppedDedup, "dedup") }
	}

	activeDelta := 0
	for _, ev := range filtered {
		switch ev.State {
		case "firing":
			activeDelta++
		case "resolved":
			activeDelta--
		}
	}
	if e.metrics != nil && activeDelta != 0 {
		e.metrics.AddActive(ctx, activeDelta, "all_rules", "mixed")
	}

	if len(filtered) > 0 && e.logsOut != nil {
		_ = e.logsOut.ConsumeLogs(ctx, buildLogs(filtered))
		if e.metrics != nil {
			e.metrics.RecordEvents(ctx, len(filtered), "all_rules", "mixed")
		}
	}
	if len(metricsView) > 0 && e.metricsOut != nil {
		_ = e.metricsOut.ConsumeMetrics(ctx, buildMetrics(metricsView))
	}

	if len(filtered) > 0 && len(e.cfg.Notify.AlertmanagerURLs) > 0 {
		n := notify.New(e.set.TelemetrySettings, notify.Config{
			Endpoints: e.cfg.Notify.AlertmanagerURLs,
			Timeout:   e.cfg.Notify.Timeout,
		})
		payload := make([]notify.AlertEvent, 0, len(filtered))
		for _, ev := range filtered {
			payload = append(payload, notify.AlertEvent{
				Rule: ev.Rule, State: ev.State, Severity: ev.Severity,
				Labels: ev.Labels, Value: ev.Value, Window: ev.Window, For: ev.For,
			})
		}
		_ = n.Notify(payload)
		if e.metrics != nil { e.metrics.RecordNotify(ctx, true) }
	}
}

func (e *Engine) ConsumeTraces(_ context.Context, td ptrace.Traces) error  { return e.ingestTraces(td) }
func (e *Engine) ConsumeLogs(_ context.Context, ld plog.Logs) error        { return e.ingestLogs(ld) }
func (e *Engine) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error { return e.ingestMetrics(md) }

func (e *Engine) ingestTraces(td ptrace.Traces) error  { return e.ing.consumeTraces(td) }
func (e *Engine) ingestLogs(ld plog.Logs) error        { return e.ing.consumeLogs(ld) }
func (e *Engine) ingestMetrics(md pmetric.Metrics) error { return e.ing.consumeMetrics(md) }

// Connector pair structs and create functions

type t2l struct{ eng *Engine }
type t2m struct{ eng *Engine }
type l2l struct{ eng *Engine }
type l2m struct{ eng *Engine }
type m2m struct{ eng *Engine }
type m2l struct{ eng *Engine }

func createTracesToLogs(_ context.Context, set connector.CreateSettings, cfg component.Config, next consumer.Logs) (connector.TracesToLogs, error) {
	e, _, err := getOrCreateEngine(cfg.(*Config), set); if err != nil { return nil, err }
	e.SetLogsConsumer(next); return &t2l{eng: e}, nil
}
func (c *t2l) Start(ctx context.Context, host component.Host) error { return c.eng.Start(ctx, host) }
func (c *t2l) Shutdown(ctx context.Context) error                   { return c.eng.Shutdown(ctx) }
func (c *t2l) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return c.eng.ConsumeTraces(ctx, td)
}

func createTracesToMetrics(_ context.Context, set connector.CreateSettings, cfg component.Config, next consumer.Metrics) (connector.TracesToMetrics, error) {
	e, _, err := getOrCreateEngine(cfg.(*Config), set); if err != nil { return nil, err }
	e.SetMetricsConsumer(next); return &t2m{eng: e}, nil
}
func (c *t2m) Start(ctx context.Context, host component.Host) error { return c.eng.Start(ctx, host) }
func (c *t2m) Shutdown(ctx context.Context) error                   { return c.eng.Shutdown(ctx) }
func (c *t2m) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return c.eng.ConsumeTraces(ctx, td)
}

func createLogsToLogs(_ context.Context, set connector.CreateSettings, cfg component.Config, next consumer.Logs) (connector.LogsToLogs, error) {
	e, _, err := getOrCreateEngine(cfg.(*Config), set); if err != nil { return nil, err }
	e.SetLogsConsumer(next); return &l2l{eng: e}, nil
}
func (c *l2l) Start(ctx context.Context, host component.Host) error { return c.eng.Start(ctx, host) }
func (c *l2l) Shutdown(ctx context.Context) error                   { return c.eng.Shutdown(ctx) }
func (c *l2l) ConsumeLogs(ctx context.Context, ld plog.Logs) error  { return c.eng.ConsumeLogs(ctx, ld) }

func createLogsToMetrics(_ context.Context, set connector.CreateSettings, cfg component.Config, next consumer.Metrics) (connector.LogsToMetrics, error) {
	e, _, err := getOrCreateEngine(cfg.(*Config), set); if err != nil { return nil, err }
	e.SetMetricsConsumer(next); return &l2m{eng: e}, nil
}
func (c *l2m) Start(ctx context.Context, host component.Host) error { return c.eng.Start(ctx, host) }
func (c *l2m) Shutdown(ctx context.Context) error                   { return c.eng.Shutdown(ctx) }
func (c *l2m) ConsumeLogs(ctx context.Context, ld plog.Logs) error  { return c.eng.ConsumeLogs(ctx, ld) }

func createMetricsToMetrics(_ context.Context, set connector.CreateSettings, cfg component.Config, next consumer.Metrics) (connector.MetricsToMetrics, error) {
	e, _, err := getOrCreateEngine(cfg.(*Config), set); if err != nil { return nil, err }
	e.SetMetricsConsumer(next); return &m2m{eng: e}, nil
}
func (c *m2m) Start(ctx context.Context, host component.Host) error { return c.eng.Start(ctx, host) }
func (c *m2m) Shutdown(ctx context.Context) error                   { return c.eng.Shutdown(ctx) }
func (c *m2m) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return c.eng.ConsumeMetrics(ctx, md)
}

func createMetricsToLogs(_ context.Context, set connector.CreateSettings, cfg component.Config, next consumer.Logs) (connector.MetricsToLogs, error) {
	e, _, err := getOrCreateEngine(cfg.(*Config), set); if err != nil { return nil, err }
	e.SetLogsConsumer(next); return &m2l{eng: e}, nil
}
func (c *m2l) Start(ctx context.Context, host component.Host) error { return c.eng.Start(ctx, host) }
func (c *m2l) Shutdown(ctx context.Context) error                   { return c.eng.Shutdown(ctx) }
func (c *m2l) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return c.eng.ConsumeMetrics(ctx, md)
}
