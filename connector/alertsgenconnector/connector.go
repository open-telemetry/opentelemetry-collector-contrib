package alertsgenconnector

import (
    "context"
    "sync"
    "time"

    "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/dedup"
    "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/storm"
    "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/cardinality"
    "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/state"

    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/connector"
    "go.opentelemetry.io/collector/consumer"
    "go.opentelemetry.io/collector/pdata/plog"
    "go.opentelemetry.io/collector/pdata/pmetric"
    "go.opentelemetry.io/collector/pdata/ptrace"
)

type Engine struct {
    mu sync.RWMutex

    cfg *Config
    set connector.CreateSettings

    logsOut    consumer.Logs
    metricsOut consumer.Metrics

    ing *ingester
    rs  *ruleSet

    limiter *storm.Limiter
    card    *cardinality.Limiter
    deduper *dedup.Deduper
    syncer  *state.TSDBSyncer

    tickCh  chan struct{}
    stopCh  chan struct{}
    started bool

    notifier *AlertManager
}

func NewEngine(cfg *Config, set connector.CreateSettings) (*Engine, error) {
    if err := cfg.Validate(); err != nil { return nil, err }

    rs, err := compileRules(cfg)
    if err != nil { return nil, err }

    e := &Engine{
        cfg: cfg,
        set: set,
        ing: newIngester(),
        rs:  rs,
        limiter: storm.NewLimiter(cfg.Storm.MaxAlertsPerMinute),
        card: cardinality.NewLimiter(cfg.Cardinality.MaxLabelsPerAlert, cfg.Cardinality.HashIfExceeds),
        deduper: dedup.NewDeduper(cfg.Dedup.Window, cfg.Dedup.FingerprintLabels, cfg.Dedup.ExcludeLabels),
        tickCh: make(chan struct{}, 1),
        stopCh: make(chan struct{}),
    }

    if cfg.TSDB.QueryURL != "" {
        e.syncer = &state.TSDBSyncer{
            QueryURL: cfg.TSDB.QueryURL,
            InstanceID: cfg.InstanceID,
            QueryInterval: cfg.TSDB.QueryInterval,
            TakeoverTimeout: cfg.TSDB.TakeoverTimeout,
        }
    }
    if len(cfg.Notify.AlertmanagerURLs) > 0 {
        e.notifier = NewAlertManager(cfg.Notify.AlertmanagerURLs, cfg.Notify.Timeout)
    }
    return e, nil
}

func (e *Engine) SetLogsConsumer(next consumer.Logs)   { e.mu.Lock(); e.logsOut = next; e.mu.Unlock() }
func (e *Engine) SetMetricsConsumer(next consumer.Metrics) { e.mu.Lock(); e.metricsOut = next; e.mu.Unlock() }

func (e *Engine) Start(ctx context.Context, host component.Host) error {
    e.mu.Lock()
    if e.started { e.mu.Unlock(); return nil }
    e.started = true
    e.mu.Unlock()

    // restore/prefetch TSDB state (non-blocking best-effort)
    go func() {
        if e.syncer != nil {
            _ = e.rs.restoreFromTSDB(e.syncer)
        }
    }()

    go e.loop()
    return nil
}

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

func (e *Engine) Shutdown(ctx context.Context) error {
    close(e.stopCh)
    return nil
}

func (e *Engine) Close() { _ = e.Shutdown(context.Background()) }

func (e *Engine) evaluateOnce() {
    now := time.Now()

    // Evaluate rules
    events, metrics := e.rs.evaluate(now, e.ing)

    // Apply storm & cardinality & dedup before emit/notify
    filtered := make([]alertEvent, 0, len(events))
    for _, ev := range events {
        if !e.limiter.Allow() {
            continue
        }
        ev.Labels = e.card.Enforce(ev.Labels)
        if !e.deduper.Allow(ev.Rule, ev.Labels) {
            continue
        }
        filtered = append(filtered, ev)
    }

    // Emit to downstream pipelines
    if len(filtered) > 0 && e.logsOut != nil {
        _ = e.logsOut.ConsumeLogs(context.Background(), buildLogs(filtered))
    }
    if len(metrics) > 0 && e.metricsOut != nil {
        _ = e.metricsOut.ConsumeMetrics(context.Background(), buildMetrics(metrics))
    }

    // Notify AlertManager (best effort, do after pipeline emit)
    if e.notifier != nil && len(filtered) > 0 {
        _ = e.notifier.Notify(filtered)
    }
}

func (e *Engine) ConsumeTraces(ctx context.Context, td ptrace.Traces) error { return e.ing.consumeTraces(td) }
func (e *Engine) ConsumeLogs(ctx context.Context, ld plog.Logs) error { return e.ing.consumeLogs(ld) }
func (e *Engine) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error { return e.ing.consumeMetrics(md) }
