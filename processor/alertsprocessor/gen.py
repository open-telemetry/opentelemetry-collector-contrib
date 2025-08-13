# Re-create the latest repo and ZIP it again so you can download fresh.
import os, pathlib, textwrap, zipfile

base = "./alertsprocessor-repo-latest"
def write(path, content):
    p = pathlib.Path(base) / path
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(content, encoding="utf-8")

# go.mod
write("go.mod", """module github.com/platformbuilds/alertsprocessor

go 1.23

require (
\tgo.opentelemetry.io/collector v0.102.0 // indirect
\tgopkg.in/yaml.v3 v3.0.1
\tgo.uber.org/zap v1.27.0
)
""")

# README with usage + routingconnector guidance
write("README.md", textwrap.dedent("""\
    > **⚠️ Sliding window cost**
    > Increasing `sliding_window.duration` increases CPU and memory usage. Keep it as small as possible (start at **5s**).

    # alertsprocessor (OpenTelemetry Collector processor)
    Evaluates alert rules over a short sliding window for **metrics, logs, and traces**, emits synthetic metrics (`otel_alert_*`), and sends notifications to a webhook (e.g., Alertmanager-compatible).

    ## Example rules.yaml (logs & traces)
    ```yaml
    - id: high_error_logs
      name: HighErrorLogs
      signal: logs
      for: 0s
      labels: { severity: error }
      logs:
        severity_at_least: ERROR
        body_contains: "timeout"
        group_by: ["service.name"]
        count_threshold: 5

    - id: slow_spans
      name: SlowSpans
      signal: traces
      for: 5s
      labels: { severity: warning }
      traces:
        latency_ms_gt: 500
        status_not_ok: false
        group_by: ["service.name","span.name"]
        count_threshold: 3
    ```

    ---

    ## How to use `alertsprocessor` in an OpenTelemetry Collector

    > **Signals supported:** Logs and Traces (rule evaluation) + synthetic metrics output.  
    > **Heads up:** Increasing `sliding_window.duration` raises CPU and memory usage.

    ### 1) Add the processor to your Collector build

    **Option A — Drop into a custom distro (simplest for testing):**
    - Place this repo under `processor/alertsprocessor/` in your Collector source tree (for example, a fork of `opentelemetry-collector-contrib`).
    - Run `make gotidy && make build` (or use the OTel Collector Builder (OCB) to include this component in a custom distribution).

    **Option B — Use OCB (OpenTelemetry Collector Builder):**
    - Create a `builder-config.yaml` that includes this processor module.
    - Build your distro with `ocb --config builder-config.yaml`.
    - See OTel docs for custom collectors and OCB usage.

    ### 2) Write alert rules (YAML)

    Create one or more rule files and point the processor at them via `rule_files.include` globs.

    ```yaml
    # rules/payments.yaml
    - id: high_error_logs
      name: HighErrorLogs
      signal: logs
      for: 0s
      labels: { severity: error }
      logs:
        severity_at_least: ERROR
        body_contains: "timeout"
        group_by: ["service.name"]
        count_threshold: 5

    - id: slow_spans
      name: SlowSpans
      signal: traces
      for: 5s
      labels: { severity: warning }
      traces:
        latency_ms_gt: 500
        status_not_ok: false
        group_by: ["service.name","span.name"]
        count_threshold: 3
    ```

    ### 3) Recommended topology: **routingconnector** + per-group `alertsprocessor`

    Use the **Routing Connector** to route telemetry by resource attributes into **separate pipelines**, each with its own `alertsprocessor` instance and its own rule files. This keeps groups isolated and makes scaling clean.

    ```yaml
    receivers:
      otlp:
        protocols: { http: {}, grpc: {} }

    connectors:
      routing:
        # Send unmatched data to the default pipelines
        default_pipelines:
          logs:   [logs/default]
          traces: [traces/default]
          metrics: [metrics/default]
        table:
          # Route by resource attribute (OTTL). Example: Kubernetes namespace
          - statement: route() where resource.attributes["k8s.namespace.name"] == "payments"
            pipelines:
              logs:   [logs/payments]
              traces: [traces/payments]
              metrics: [metrics/payments]
          - statement: route() where resource.attributes["service.name"] == "checkout"
            pipelines:
              logs:   [logs/checkout]
              traces: [traces/checkout]

    processors:
      # Per-group alerts engine instances (distinct rule sets & labels)
      alertsprocessor/payments:
        sliding_window:
          duration: 5s    # ⚠️ Larger window ⇒ higher CPU & RAM
          max_samples: 100000
          overflow_behavior: ring_buffer
        evaluation:
          interval: 15s
          timeout: 10s
        statestore:
          instance_id: payments-engine
          external_labels:
            group: payments
            source: collector
        rule_files:
          include: ["./rules/payments.yaml"]
        notifier:
          url: http://alertmanager:9093/api/v2/alerts

      alertsprocessor/checkout:
        sliding_window:
          duration: 5s
          max_samples: 100000
          overflow_behavior: ring_buffer
        evaluation:
          interval: 15s
          timeout: 10s
        statestore:
          instance_id: checkout-engine
          external_labels:
            group: checkout
            source: collector
        rule_files:
          include: ["./rules/checkout.yaml"]
        notifier:
          url: http://alertmanager:9093/api/v2/alerts

    exporters:
      prometheusremotewrite:
        endpoint: http://prometheus:9090/api/v1/write
      debug: {}

    service:
      pipelines:
        # Ingress pipelines export to the routing connector
        logs/in:
          receivers: [otlp]
          exporters: [routing]
        traces/in:
          receivers: [otlp]
          exporters: [routing]
        metrics/in:
          receivers: [otlp]
          exporters: [routing]

        # Grouped pipelines receive from routing, then run their own alertsprocessor
        logs/payments:
          receivers: [routing]
          processors: [alertsprocessor/payments]
          exporters: [debug, prometheusremotewrite]

        traces/payments:
          receivers: [routing]
          processors: [alertsprocessor/payments]
          exporters: [debug, prometheusremotewrite]

        logs/checkout:
          receivers: [routing]
          processors: [alertsprocessor/checkout]
          exporters: [debug, prometheusremotewrite]

        traces/checkout:
          receivers: [routing]
          processors: [alertsprocessor/checkout]
          exporters: [debug, prometheusremotewrite]

        # Default pipelines for unmatched telemetry (optional)
        logs/default:
          receivers: [routing]
          exporters: [debug]
        traces/default:
          receivers: [routing]
          exporters: [debug]
        metrics/default:
          receivers: [routing]
          exporters: [debug]
    ```

    **Why this layout?**
    - The routing connector is configured in the `connectors:` block and is used as an **exporter** from ingress pipelines and as a **receiver** for the destination pipelines.  
    - Each destination pipeline runs a **separate `alertsprocessor` instance** with its own rules (`rule_files.include`), labels, and notifier.  
    - This keeps state, notifications, and alert metrics **segregated per group**, making it easy to scale horizontally by adding more routes and processor instances.

    ### 4) Output series (Prometheus Remote Write)

    The processor emits synthetic metrics describing alert state and transitions, which you can send to any Prometheus remote write endpoint:

    - `otel_alert_state{rule_id, signal, ...} = 0|1` (gauge)
    - `otel_alert_transitions_total{rule_id, from, to, signal, ...}` (counter)
    - `alertsprocessor_evaluation_duration_seconds` (self-telemetry)

    ### Operational notes

    - **Sliding window cost:** Increasing `sliding_window.duration` increases both CPU and memory usage. Start with **5s** and raise only if necessary.
    - **Log body type:** If a log record’s `Body` is not a string, the processor logs a **WARN** and stringifies it for `body_contains` matching.
    - **Rules scope:** Current rules target **logs** and **traces**. (Metric-rule evaluation can be added following the same interface.)
    - **Statestore:** Use `statestore.instance_id` and `statestore.external_labels` to distinguish multiple engines in the same Collector.

    ### References
    - Use the **Routing Connector** to keep groups segregated and scalable: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/routingconnector
    """))

# metadata.yaml
write("processor/alertsprocessor/metadata.yaml", textwrap.dedent("""\
    type: processor
    status:
      class: processor
      stability:
        development: [metrics, logs, traces]
      distributions: [contrib]

    telemetry:
      metrics:
        alertsprocessor_evaluation_duration_seconds:
          description: Duration of a single evaluation tick
          unit: s
          histogram:
            buckets: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5]
          enabled: true
        alertsprocessor_notifications_total:
          description: Count of notifications attempted by result
          unit: 1
          sum:
            value_type: int
            monotonic: true
          attributes:
            - name: result
              description: sent|failed|dropped
            - name: endpoint
              description: notifier url host
          enabled: true
        alertsprocessor_state_sync_errors_total:
          description: Errors during state synchronization with statestore
          unit: 1
          sum:
            value_type: int
            monotonic: true
          enabled: true
    """))

# config.go
write("processor/alertsprocessor/config.go", textwrap.dedent("""\
    package alertsprocessor

    import (
        "errors"
        "fmt"
        "time"

        "go.opentelemetry.io/collector/component"
        "go.opentelemetry.io/collector/config"
        "go.opentelemetry.io/collector/config/confighttp"
    )

    type SlidingWindowConfig struct {
        Duration         time.Duration `mapstructure:"duration"`
        MaxSamples       int           `mapstructure:"max_samples"`
        OverflowBehavior string        `mapstructure:"overflow_behavior"` // ring_buffer | drop_new
    }

    type EvaluationConfig struct {
        Interval      time.Duration `mapstructure:"interval"`
        Timeout       time.Duration `mapstructure:"timeout"`
        MaxConcurrent int           `mapstructure:"max_concurrent"`
    }

    type StatestoreConfig struct {
        RemoteRead struct {
            URL string `mapstructure:"url"`
        } `mapstructure:"remote_read"`
        RemoteWrite struct {
            URL string `mapstructure:"url"`
        } `mapstructure:"remote_write"`
        SyncInterval   time.Duration     `mapstructure:"sync_interval"`
        InstanceID     string            `mapstructure:"instance_id"`
        ExternalLabels map[string]string `mapstructure:"external_labels"`
        ExternalURL    string            `mapstructure:"external_url"`
    }

    type DedupConfig struct {
        FingerprintAlgorithm string   `mapstructure:"fingerprint_algorithm"` // sha256
        FingerprintLabels    []string `mapstructure:"fingerprint_labels"`
        ExcludeLabels        []string `mapstructure:"exclude_labels"`
    }

    type StormControlConfig struct {
        Global struct {
            MaxActiveAlerts         int     `mapstructure:"max_active_alerts"`
            MaxAlertsPerMinute      int     `mapstructure:"max_alerts_per_minute"`
            CircuitBreakerThreshold float64 `mapstructure:"circuit_breaker_threshold"`
        } `mapstructure:"global"`
    }

    type CardinalityLabels struct {
        MaxLabelsPerAlert   int `mapstructure:"max_labels_per_alert"`
        MaxLabelValueLength int `mapstructure:"max_label_value_length"`
        MaxTotalLabelSize   int `mapstructure:"max_total_label_size"`
    }

    type CardinalitySeries struct {
        MaxActiveSeries  int `mapstructure:"max_active_series"`
        MaxSeriesPerRule int `mapstructure:"max_series_per_rule"`
    }

    type CardinalityConfig struct {
        Labels        CardinalityLabels `mapstructure:"labels"`
        Allowlist     []string          `mapstructure:"allowlist"`
        Blocklist     []string          `mapstructure:"blocklist"`
        HashIfExceeds int               `mapstructure:"hash_if_exceeds"`
        HashAlgorithm string            `mapstructure:"hash_algorithm"`
        Series        CardinalitySeries `mapstructure:"series"`
        Enforcement   struct {
            Mode           string `mapstructure:"mode"`
            OverflowAction string `mapstructure:"overflow_action"`
        } `mapstructure:"enforcement"`
    }

    type NotifierConfig struct {
        URL             string                  `mapstructure:"url"`
        HTTPClient      confighttp.ClientConfig `mapstructure:",squash"`
        Timeout         time.Duration           `mapstructure:"timeout"`
        InitialInterval time.Duration           `mapstructure:"initial_interval"`
        MaxInterval     time.Duration           `mapstructure:"max_interval"`
        MaxBatchSize    int                     `mapstructure:"max_batch_size"`
        DisableSending  bool                    `mapstructure:"disable_sending"`
    }

    type RuleFiles struct {
        Include []string `mapstructure:"include"`
    }

    type Config struct {
        config.ProcessorSettings `mapstructure:",squash"`

        SlidingWindow SlidingWindowConfig `mapstructure:"sliding_window"`
        Evaluation    EvaluationConfig    `mapstructure:"evaluation"`
        Statestore    StatestoreConfig    `mapstructure:"statestore"`
        Dedup         DedupConfig         `mapstructure:"deduplication"`
        StormControl  StormControlConfig  `mapstructure:"stormcontrol"`
        Cardinality   CardinalityConfig   `mapstructure:"cardinality"`
        Notifier      NotifierConfig      `mapstructure:"notifier"`

        RuleFiles RuleFiles `mapstructure:"rule_files"`
    }

    func createDefaultConfig() component.Config {
        return &Config{
            ProcessorSettings: config.NewProcessorSettings(component.MustNewID(typeStr)),
            SlidingWindow: SlidingWindowConfig{Duration: 5 * time.Second, MaxSamples: 100_000, OverflowBehavior: "ring_buffer"},
            Evaluation:    EvaluationConfig{Interval: 15 * time.Second, Timeout: 10 * time.Second, MaxConcurrent: 0},
            Statestore:    StatestoreConfig{SyncInterval: 30 * time.Second},
            Notifier:      NotifierConfig{Timeout: 5 * time.Second, InitialInterval: 500 * time.Millisecond, MaxInterval: 30 * time.Second, MaxBatchSize: 64},
            RuleFiles:     RuleFiles{Include: nil},
        }
    }

    func (c *Config) Validate() error {
        if c.SlidingWindow.Duration <= 0 {
            return errors.New("sliding_window.duration must be > 0")
        }
        if c.Evaluation.Interval <= 0 {
            return errors.New("evaluation.interval must be > 0")
        }
        if c.SlidingWindow.OverflowBehavior != "ring_buffer" && c.SlidingWindow.OverflowBehavior != "drop_new" {
            return fmt.Errorf("sliding_window.overflow_behavior must be ring_buffer or drop_new")
        }
        return nil
    }
    """))

# factory.go
write("processor/alertsprocessor/factory.go", textwrap.dedent("""\
    package alertsprocessor

    import (
        "context"
        "time"

        "go.opentelemetry.io/collector/component"
        "go.opentelemetry.io/collector/consumer"
        "go.opentelemetry.io/collector/processor"
        "go.opentelemetry.io/collector/processor/processorhelper"
    )

    const typeStr = "alertsprocessor"

    func NewFactory() processor.Factory {
        return processor.NewFactory(
            component.MustNewType(typeStr),
            createDefaultConfig,
            processor.WithMetrics(createMetrics, component.StabilityLevelAlpha),
            processor.WithLogs(createLogs, component.StabilityLevelAlpha),
            processor.WithTraces(createTraces, component.StabilityLevelAlpha),
        )
    }

    func createMetrics(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Metrics) (processor.Metrics, error) {
        c := cfg.(*Config)
        if err := c.Validate(); err != nil { return nil, err }
        p, err := newProcessor(ctx, set, c, next, nil, nil)
        if err != nil { return nil, err }
        return processorhelper.NewMetrics(
            ctx, set, cfg, next, p.processMetrics,
            processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
            processorhelper.WithStart(p.Start),
            processorhelper.WithShutdown(p.Shutdown),
            processorhelper.WithTimeouts(processorhelper.Timeouts{Process: c.Evaluation.Timeout, Shutdown: 5 * time.Second}),
        )
    }

    func createLogs(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Logs) (processor.Logs, error) {
        c := cfg.(*Config)
        if err := c.Validate(); err != nil { return nil, err }
        p, err := newProcessor(ctx, set, c, nil, next, nil)
        if err != nil { return nil, err }
        return processorhelper.NewLogs(
            ctx, set, cfg, next, p.processLogs,
            processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
            processorhelper.WithStart(p.Start),
            processorhelper.WithShutdown(p.Shutdown),
            processorhelper.WithTimeouts(processorhelper.Timeouts{Process: c.Evaluation.Timeout, Shutdown: 5 * time.Second}),
        )
    }

    func createTraces(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Traces) (processor.Traces, error) {
        c := cfg.(*Config)
        if err := c.Validate(); err != nil { return nil, err }
        p, err := newProcessor(ctx, set, c, nil, nil, next)
        if err != nil { return nil, err }
        return processorhelper.NewTraces(
            ctx, set, cfg, next, p.processTraces,
            processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
            processorhelper.WithStart(p.Start),
            processorhelper.WithShutdown(p.Shutdown),
            processorhelper.WithTimeouts(processorhelper.Timeouts{Process: c.Evaluation.Timeout, Shutdown: 5 * time.Second}),
        )
    }
    """))

# processor.go
write("processor/alertsprocessor/processor.go", textwrap.dedent("""\
    package alertsprocessor

    import (
        "context"
        "sync"
        "time"

        "go.opentelemetry.io/collector/component"
        "go.opentelemetry.io/collector/consumer"
        "go.opentelemetry.io/collector/pdata/pcommon"
        "go.opentelemetry.io/collector/pdata/plog"
        "go.opentelemetry.io/collector/pdata/pmetric"
        "go.opentelemetry.io/collector/pdata/ptrace"
        "go.opentelemetry.io/collector/processor"

        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/cardinality"
        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/evaluation"
        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/notify"
        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/output"
        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/slidingwindow"
        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/statestore"
        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/stormcontrol"
    )

    type processorImpl struct {
        cfg   *Config
        set   processor.Settings
        nextM consumer.Metrics
        nextL consumer.Logs
        nextT consumer.Traces

        win   *slidingwindow.Window
        eval  *evaluation.Engine
        store *statestore.Store
        card  *cardinality.Limiter
        gov   *stormcontrol.Governor
        notif *notify.Notifier
        out   *output.SeriesBuilder

        tick   *time.Ticker
        stopCh chan struct{}
        wg     sync.WaitGroup
    }

    func newProcessor(ctx context.Context, set processor.Settings, cfg *Config, m consumer.Metrics, l consumer.Logs, t consumer.Traces) (*processorImpl, error) {
        w := slidingwindow.New(cfg.SlidingWindow)
        e := evaluation.NewEngine(evaluation.RuleFiles{Include: cfg.RuleFiles.Include}, set.Logger)
        st := statestore.New(cfg.Statestore, set.Logger)
        cd := cardinality.New(cfg.Cardinality)
        gv := stormcontrol.New(cfg.StormControl)
        nf := notify.New(cfg.Notifier, set.Logger)
        ob := output.NewSeriesBuilder(cfg.Statestore.ExternalLabels)

        return &processorImpl{
            cfg:   cfg,
            set:   set,
            nextM: m, nextL: l, nextT: t,
            win:   w,
            eval:  e,
            store: st,
            card:  cd,
            gov:   gv,
            notif: nf,
            out:   ob,
            stopCh: make(chan struct{}),
        }, nil
    }

    func (p *processorImpl) Start(ctx context.Context, _ component.Host) error {
        if p.cfg.SlidingWindow.Duration > 15*time.Second {
            p.set.Logger.Warn("Large sliding_window.duration increases CPU and memory usage; consider keeping it small", "duration", p.cfg.SlidingWindow.Duration)
        }
        p.tick = time.NewTicker(p.cfg.Evaluation.Interval)
        p.wg.Add(1)
        go p.loop(ctx)
        return nil
    }

    func (p *processorImpl) Shutdown(ctx context.Context) error {
        close(p.stopCh)
        if p.tick != nil { p.tick.Stop() }
        p.wg.Wait()
        return nil
    }

    func (p *processorImpl) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) { p.win.IngestMetrics(md); return md, nil }
    func (p *processorImpl) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error)               { p.win.IngestLogs(ld);    return ld, nil }
    func (p *processorImpl) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error)     { p.win.IngestTraces(td);  return td, nil }

    func (p *processorImpl) loop(ctx context.Context) {
        defer p.wg.Done()
        for {
            select {
            case <-p.stopCh:
                return
            case <-p.tick.C:
                p.evaluate(ctx, time.Now())
            }
        }
    }

    func (p *processorImpl) evaluate(ctx context.Context, ts time.Time) {
        start := time.Now()

        mets := p.win.SnapshotMetrics()
        logs := p.win.SnapshotLogs()
        trcs := p.win.SnapshotTraces()

        var results []evaluation.Result
        results = append(results, p.eval.RunMetrics(mets, ts)...)
        results = append(results, p.eval.RunLogs(logs, ts)...)
        results = append(results, p.eval.RunTraces(trcs, ts)...)

        for i := range results { results[i] = p.card.FilterResult(results[i]) }

        transitions := p.store.Apply(results, ts)

        md := p.out.Build(results, transitions, ts)
        if md.DataPointCount() > 0 && p.nextM != nil { _ = p.nextM.ConsumeMetrics(ctx, md) }

        p.notif.Notify(ctx, transitions)

        p.emitEvalDuration(ctx, time.Since(start), ts)

        p.gov.Adapt(&p.tick, results, ts)
    }

    func (p *processorImpl) emitEvalDuration(ctx context.Context, d time.Duration, ts time.Time) {
        if p.nextM == nil { return }
        md := pmetric.NewMetrics()
        rm := md.ResourceMetrics().AppendEmpty()
        sm := rm.ScopeMetrics().AppendEmpty()
        m := sm.Metrics().AppendEmpty()
        m.SetName("alertsprocessor_evaluation_duration_seconds")
        m.SetEmptyGauge()
        dp := m.Gauge().DataPoints().AppendEmpty()
        dp.SetDoubleValue(d.Seconds())
        dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
        _ = p.nextM.ConsumeMetrics(ctx, md)
    }
    """))

# slidingwindow
write("processor/alertsprocessor/slidingwindow/window.go", textwrap.dedent("""\
    package slidingwindow

    import (
        "sync"
        "time"

        "go.opentelemetry.io/collector/pdata/plog"
        "go.opentelemetry.io/collector/pdata/pmetric"
        "go.opentelemetry.io/collector/pdata/ptrace"

        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor"
    )

    type Window struct {
        cfg alertsprocessor.SlidingWindowConfig
        mu  sync.RWMutex

        mets []tm[pmetric.Metrics]
        logs []tm[plog.Logs]
        trcs []tm[ptrace.Traces]
    }

    type tm[T any] struct {
        at time.Time
        v  T
    }

    func New(cfg alertsprocessor.SlidingWindowConfig) *Window { return &Window{cfg: cfg} }

    func (w *Window) IngestMetrics(md pmetric.Metrics) { w.mu.Lock(); w.mets = append(w.mets, tm[pmetric.Metrics]{at: time.Now(), v: md}); w.trim(); w.mu.Unlock() }
    func (w *Window) IngestLogs(ld plog.Logs)          { w.mu.Lock(); w.logs = append(w.logs, tm[plog.Logs]{at: time.Now(), v: ld}); w.trim(); w.mu.Unlock() }
    func (w *Window) IngestTraces(td ptrace.Traces)    { w.mu.Lock(); w.trcs = append(w.trcs, tm[ptrace.Traces]{at: time.Now(), v: td}); w.trim(); w.mu.Unlock() }

    func (w *Window) SnapshotMetrics() []pmetric.Metrics {
        w.mu.RLock(); defer w.mu.RUnlock()
        cut := time.Now().Add(-w.cfg.Duration)
        out := make([]pmetric.Metrics, 0, len(w.mets))
        for _, x := range w.mets { if x.at.After(cut) { out = append(out, x.v) } }
        return out
    }
    func (w *Window) SnapshotLogs() []plog.Logs {
        w.mu.RLock(); defer w.mu.RUnlock()
        cut := time.Now().Add(-w.cfg.Duration)
        out := make([]plog.Logs, 0, len(w.logs))
        for _, x := range w.logs { if x.at.After(cut) { out = append(out, x.v) } }
        return out
    }
    func (w *Window) SnapshotTraces() []ptrace.Traces {
        w.mu.RLock(); defer w.mu.RUnlock()
        cut := time.Now().Add(-w.cfg.Duration)
        out := make([]ptrace.Traces, 0, len(w.trcs))
        for _, x := range w.trcs { if x.at.After(cut) { out = append(out, x.v) } }
        return out
    }

    func (w *Window) trim() {
        cut := time.Now().Add(-w.cfg.Duration)
        crop := func[T any](buf []tm[T]) []tm[T] {
            i := 0
            for ; i < len(buf); i++ {
                if buf[i].at.After(cut) { break }
            }
            if i > 0 { buf = append([]tm[T]{}, buf[i:]...) }
            if w.cfg.MaxSamples > 0 && len(buf) > w.cfg.MaxSamples {
                if w.cfg.OverflowBehavior == "ring_buffer" {
                    buf = buf[len(buf)-w.cfg.MaxSamples:]
                } else {
                    buf = buf[:w.cfg.MaxSamples]
                }
            }
            return buf
        }
        w.mets = crop(w.mets)
        w.logs = crop(w.logs)
        w.trcs = crop(w.trcs)
    }
    """))

# evaluation engine with WARN on non-string log bodies
write("processor/alertsprocessor/evaluation/engine.go", textwrap.dedent("""\
    package evaluation

    import (
        "os"
        "path/filepath"
        "sort"
        "strings"
        "time"

        "go.opentelemetry.io/collector/pdata/pcommon"
        "go.opentelemetry.io/collector/pdata/plog"
        "go.opentelemetry.io/collector/pdata/pmetric"
        "go.opentelemetry.io/collector/pdata/ptrace"
        "go.uber.org/zap"
        "gopkg.in/yaml.v3"
    )

    type RuleFiles struct {
        Include []string
    }

    type Rule struct {
        ID          string            `yaml:"id"`
        Name        string            `yaml:"name"`
        Signal      string            `yaml:"signal"` // metrics|logs|traces
        For         time.Duration     `yaml:"for"`
        Labels      map[string]string `yaml:"labels"`
        Annotations map[string]string `yaml:"annotations"`

        Logs   *LogsRule   `yaml:"logs,omitempty"`
        Traces *TracesRule `yaml:"traces,omitempty"`
    }

    type LogsRule struct {
        SeverityAtLeast string            `yaml:"severity_at_least"` // DEBUG|INFO|WARN|ERROR
        BodyContains    string            `yaml:"body_contains"`
        AttrEquals      map[string]string `yaml:"attr_equals"`
        GroupBy         []string          `yaml:"group_by"`
        CountThreshold  int               `yaml:"count_threshold"`
    }

    type TracesRule struct {
        LatencyMillisGT int               `yaml:"latency_ms_gt"`
        StatusNotOK     bool              `yaml:"status_not_ok"`
        AttrEquals      map[string]string `yaml:"attr_equals"`
        GroupBy         []string          `yaml:"group_by"`
        CountThreshold  int               `yaml:"count_threshold"`
    }

    type Instance struct {
        RuleID      string
        Fingerprint string
        Labels      map[string]string
        Active      bool
        Value       float64
    }

    type Result struct {
        Rule      Rule
        At        time.Time
        Signal    string
        Instances []Instance
    }

    type Engine struct {
        log         *zap.Logger
        logsRules   []Rule
        tracesRules []Rule
    }

    func NewEngine(files RuleFiles, log *zap.Logger) *Engine {
        e := &Engine{log: log}
        rules := loadRules(files, log)
        for _, r := range rules {
            switch strings.ToLower(r.Signal) {
            case "logs":
                e.logsRules = append(e.logsRules, r)
            case "traces":
                e.tracesRules = append(e.tracesRules, r)
            case "metrics":
                // TODO: metrics rules in future
            }
        }
        return e
    }

    func (e *Engine) RunMetrics(_ []pmetric.Metrics, _ time.Time) []Result { return nil }

    func (e *Engine) RunLogs(w []plog.Logs, ts time.Time) []Result {
        if len(e.logsRules) == 0 || len(w) == 0 { return nil }
        var out []Result
        for _, r := range e.logsRules {
            counts := map[string]int{}
            labelsForKey := map[string]map[string]string{}
            for _, ld := range w {
                for i := 0; i < ld.ResourceLogs().Len(); i++ {
                    rl := ld.ResourceLogs().At(i)
                    res := rl.Resource()
                    for j := 0; j < rl.ScopeLogs().Len(); j++ {
                        sl := rl.ScopeLogs().At(j)
                        for k := 0; k < sl.LogRecords().Len(); k++ {
                            lr := sl.LogRecords().At(k)
                            if !matchLog(r.Logs, e.log, res, lr) { continue }
                            key, lab := logGroupKey(r.Logs.GroupBy, res, lr)
                            counts[key]++
                            if _, ok := labelsForKey[key]; !ok { labelsForKey[key] = lab }
                        }
                    }
                }
            }
            threshold := r.Logs.CountThreshold
            if threshold <= 0 { threshold = 1 }
            var insts []Instance
            for k, c := range counts {
                if c >= threshold {
                    insts = append(insts, Instance{
                        RuleID: r.ID,
                        Fingerprint: k,
                        Labels: merge(labelsForKey[k], map[string]string{"signal":"logs", "alertname": pick(r.Name, r.ID)}),
                        Active: true,
                        Value: float64(c),
                    })
                }
            }
            if len(insts) > 0 {
                out = append(out, Result{Rule: r, At: ts, Signal: "logs", Instances: insts})
            }
        }
        return out
    }

    func (e *Engine) RunTraces(w []ptrace.Traces, ts time.Time) []Result {
        if len(e.tracesRules) == 0 || len(w) == 0 { return nil }
        var out []Result
        for _, r := range e.tracesRules {
            counts := map[string]int{}
            labelsForKey := map[string]map[string]string{}
            for _, td := range w {
                for i := 0; i < td.ResourceSpans().Len(); i++ {
                    rs := td.ResourceSpans().At(i)
                    res := rs.Resource()
                    for j := 0; j < rs.ScopeSpans().Len(); j++ {
                        ss := rs.ScopeSpans().At(j)
                        for k := 0; k < ss.Spans().Len(); k++ {
                            sp := ss.Spans().At(k)
                            if !matchSpan(r.Traces, res, sp) { continue }
                            key, lab := traceGroupKey(r.Traces.GroupBy, res, sp)
                            counts[key]++
                            if _, ok := labelsForKey[key]; !ok { labelsForKey[key] = lab }
                        }
                    }
                }
            }
            threshold := r.Traces.CountThreshold
            if threshold <= 0 { threshold = 1 }
            var insts []Instance
            for k, c := range counts {
                if c >= threshold {
                    insts = append(insts, Instance{
                        RuleID: r.ID,
                        Fingerprint: k,
                        Labels: merge(labelsForKey[k], map[string]string{"signal":"traces", "alertname": pick(r.Name, r.ID)}),
                        Active: true,
                        Value: float64(c),
                    })
                }
            }
            if len(insts) > 0 {
                out = append(out, Result{Rule: r, At: ts, Signal: "traces", Instances: insts})
            }
        }
        return out
    }

    // Helpers

    func loadRules(files RuleFiles, log *zap.Logger) []Rule {
        var out []Rule
        for _, pat := range files.Include {
            matches, _ := filepath.Glob(pat)
            for _, path := range matches {
                b, err := os.ReadFile(path)
                if err != nil { if log != nil { log.Warn("failed to read rule file", zap.String("file", path), zap.Error(err)) }; continue }
                var list []Rule
                if err := yaml.Unmarshal(b, &list); err == nil && len(list) > 0 {
                    out = append(out, list...)
                    continue
                }
                var one Rule
                if err := yaml.Unmarshal(b, &one); err == nil && (one.Signal != "" || one.ID != "") {
                    out = append(out, one)
                }
            }
        }
        sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
        return out
    }

    func pick(a, b string) string { if strings.TrimSpace(a) != "" { return a }; return b }

    func matchLog(rule *LogsRule, log *zap.Logger, res pcommon.Resource, lr plog.LogRecord) bool {
        if rule == nil { return false }
        // WARN if Body is non-string
        if lr.Body().Type() != pcommon.ValueTypeStr && log != nil {
            log.Warn("non-string log body encountered; converting to string for body_contains",
                zap.String("body_type", lr.Body().Type().String()))
        }
        if rule.SeverityAtLeast != "" {
            if lr.SeverityNumber() < severityToNumber(rule.SeverityAtLeast) { return false }
        }
        if s := rule.BodyContains; s != "" {
            if !strings.Contains(strings.ToLower(lr.Body().AsString()), strings.ToLower(s)) { return false }
        }
        if len(rule.AttrEquals) > 0 {
            attrs := lr.Attributes()
            resAttrs := res.Attributes()
            for k, v := range rule.AttrEquals {
                if !attrEquals(attrs, resAttrs, k, v) { return false }
            }
        }
        return true
    }

    func logGroupKey(keys []string, res pcommon.Resource, lr plog.LogRecord) (string, map[string]string) {
        if len(keys) == 0 { return "all", map[string]string{} }
        attrs := lr.Attributes()
        rattrs := res.Attributes()
        parts := make([]string, 0, len(keys))
        out := map[string]string{}
        for _, k := range keys {
            val := findAttr(attrs, rattrs, k)
            parts = append(parts, k+"="+val)
            out[k] = val
        }
        return strings.Join(parts, "|"), out
    }

    func matchSpan(rule *TracesRule, res pcommon.Resource, sp ptrace.Span) bool {
        if rule == nil { return false }
        if rule.LatencyMillisGT > 0 {
            dur := int64(sp.EndTimestamp()-sp.StartTimestamp())/1_000_000
            if dur <= int64(rule.LatencyMillisGT) { return false }
        }
        if rule.StatusNotOK {
            if sp.Status().Code() == ptrace.StatusCodeOk { return false }
        }
        if len(rule.AttrEquals) > 0 {
            attrs := sp.Attributes()
            resAttrs := res.Attributes()
            for k, v := range rule.AttrEquals {
                if !attrEquals(attrs, resAttrs, k, v) { return false }
            }
        }
        return true
    }

    func traceGroupKey(keys []string, res pcommon.Resource, sp ptrace.Span) (string, map[string]string) {
        if len(keys) == 0 { return "all", map[string]string{} }
        attrs := sp.Attributes()
        rattrs := res.Attributes()
        parts := make([]string, 0, len(keys))
        out := map[string]string{}
        for _, k := range keys {
            val := ""
            if k == "span.name" { val = sp.Name() } else { val = findAttr(attrs, rattrs, k) }
            parts = append(parts, k+"="+val)
            out[k] = val
        }
        return strings.Join(parts, "|"), out
    }

    func severityToNumber(s string) plog.SeverityNumber {
        switch strings.ToUpper(strings.TrimSpace(s)) {
        case "TRACE": return plog.SeverityNumberTrace
        case "DEBUG": return plog.SeverityNumberDebug
        case "INFO": return plog.SeverityNumberInfo
        case "WARN", "WARNING": return plog.SeverityNumberWarn
        case "ERROR": return plog.SeverityNumberError
        case "FATAL": return plog.SeverityNumberFatal
        default: return plog.SeverityNumberUnspecified
        }
    }

    func attrEquals(attrs, resAttrs pcommon.Map, key, want string) bool {
        if v, ok := attrs.Get(key); ok { return v.AsString() == want }
        if v, ok := resAttrs.Get(key); ok { return v.AsString() == want }
        return false
    }

    func findAttr(attrs, resAttrs pcommon.Map, key string) string {
        if v, ok := attrs.Get(key); ok { return v.AsString() }
        if v, ok := resAttrs.Get(key); ok { return v.AsString() }
        return ""
    }

    func merge(a, b map[string]string) map[string]string {
        out := map[string]string{}
        for k, v := range a { out[k] = v }
        for k, v := range b { out[k] = v }
        return out
    }
    """))

# statestore
write("processor/alertsprocessor/statestore/syncer.go", textwrap.dedent("""\
    package statestore

    import (
        "context"
        "crypto/sha256"
        "encoding/hex"
        "sort"
        "strings"
        "sync"
        "time"

        "go.uber.org/zap"

        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/evaluation"
    )

    type Store struct {
        cfg   interface{}
        log   *zap.Logger
        mu    sync.Mutex
        state map[string]alertState // key: signal|ruleID|fp
    }

    type alertState struct {
        Signal   string
        Active   bool
        Since    time.Time
        LastEval time.Time
        Labels   map[string]string
        For      time.Duration
    }

    type Transition struct {
        Signal      string
        RuleID      string
        Fingerprint string
        From        string
        To          string
        Labels      map[string]string
        At          time.Time
    }

    func New(_cfg interface{}, log *zap.Logger) *Store {
        return &Store{cfg: _cfg, log: log, state: map[string]alertState{}}
    }

    func (s *Store) key(signal, ruleID, fp string) string { return signal + "|" + ruleID + "|" + fp }

    func (s *Store) Sync(_ context.Context, _ time.Time) { /* hook for external state */ }

    func (s *Store) Apply(results []evaluation.Result, ts time.Time) []Transition {
        s.mu.Lock(); defer s.mu.Unlock()
        var trans []Transition
        seen := map[string]bool{}

        for _, r := range results {
            for _, inst := range r.Instances {
                fp := inst.Fingerprint
                if fp == "" { fp = computeFP(inst.Labels) }
                k := s.key(r.Signal, r.Rule.ID, fp)
                seen[k] = true

                prev, ok := s.state[k]
                if !ok { prev = alertState{Signal: r.Signal, Labels: inst.Labels, For: r.Rule.For} }
                cur := prev
                cur.LastEval = ts
                cur.Labels = inst.Labels
                cur.For = r.Rule.For

                if inst.Active {
                    if !prev.Active {
                        if cur.Since.IsZero() { cur.Since = ts }
                        if cur.For <= 0 || ts.Sub(cur.Since) >= cur.For {
                            cur.Active = true
                            trans = append(trans, Transition{Signal: r.Signal, RuleID: r.Rule.ID, Fingerprint: fp, From: stateStr(prev.Active), To: "firing", Labels: inst.Labels, At: ts})
                        }
                    } else {
                        cur.Active = true
                    }
                } else {
                    if prev.Active {
                        cur.Active = false
                        trans = append(trans, Transition{Signal: r.Signal, RuleID: r.Rule.ID, Fingerprint: fp, From: "firing", To: "resolved", Labels: inst.Labels, At: ts})
                    }
                    cur.Since = time.Time{}
                }
                s.state[k] = cur
            }
        }

        for k, st := range s.state {
            if st.Active && !seen[k] {
                parts := strings.SplitN(k, "|", 3)
                trans = append(trans, Transition{
                    Signal: parts[0], RuleID: parts[1], Fingerprint: parts[2],
                    From: "firing", To: "resolved", Labels: st.Labels, At: ts,
                })
                st.Active = false
                s.state[k] = st
            }
        }
        return trans
    }

    func stateStr(active bool) string { if active { return "firing" }; return "pending" }

    func computeFP(labels map[string]string) string {
        if len(labels) == 0 { return "" }
        keys := make([]string, 0, len(labels))
        for k := range labels { keys = append(keys, k) }
        sort.Strings(keys)
        var b strings.Builder
        for _, k := range keys {
            b.WriteString(k); b.WriteString("="); b.WriteString(labels[k]); b.WriteString(",")
        }
        sum := sha256.Sum256([]byte(b.String()))
        return hex.EncodeToString(sum[:8])
    }
    """))

# cardinality
write("processor/alertsprocessor/cardinality/limiter.go", textwrap.dedent("""\
    package cardinality

    import (
        "unicode/utf8"

        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor"
        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/evaluation"
    )

    type Limiter struct { cfg alertsprocessor.CardinalityConfig }

    func New(cfg alertsprocessor.CardinalityConfig) *Limiter { return &Limiter{cfg: cfg} }

    func (l *Limiter) FilterResult(r evaluation.Result) evaluation.Result {
        for i := range r.Instances {
            r.Instances[i].Labels = l.filterLabels(r.Instances[i].Labels)
        }
        return r
    }

    func (l *Limiter) filterLabels(in map[string]string) map[string]string {
        if in == nil { return nil }
        out := make(map[string]string, len(in))
        allow := map[string]struct{}{}
        for _, k := range l.cfg.Allowlist { allow[k] = struct{}{} }
        block := map[string]struct{}{}
        for _, k := range l.cfg.Blocklist { block[k] = struct{}{} }
        for k, v := range in {
            if _, b := block[k]; b { continue }
            if len(l.cfg.Allowlist) > 0 {
                if _, ok := allow[k]; !ok { continue }
            }
            if l.cfg.Labels.MaxLabelValueLength > 0 {
                v = truncate(v, l.cfg.Labels.MaxLabelValueLength)
            }
            out[k] = v
        }
        return out
    }

    func truncate(s string, n int) string {
        if n <= 0 { return "" }
        if utf8.RuneCountInString(s) <= n { return s }
        r := []rune(s)
        return string(r[:n])
    }
    """))

# stormcontrol
write("processor/alertsprocessor/stormcontrol/governor.go", textwrap.dedent("""\
    package stormcontrol

    import (
        "time"

        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/evaluation"
    )

    type Governor struct{ cfg interface{} }

    func New(cfg interface{}) *Governor { return &Governor{cfg: cfg} }

    func (g *Governor) Adapt(_ **time.Ticker, _ []evaluation.Result, _ time.Time) {
        // no-op adapter for now
    }
    """))

# notify
write("processor/alertsprocessor/notify/payload_alertmanager.go", textwrap.dedent("""\
    package notify

    type AMAlert struct {
        Status       string            `json:"status"`
        Labels       map[string]string `json:"labels"`
        Annotations  map[string]string `json:"annotations,omitempty"`
        StartsAt     string            `json:"startsAt"`
        EndsAt       string            `json:"endsAt,omitempty"`
        GeneratorURL string            `json:"generatorURL,omitempty"`
    }
    """))

write("processor/alertsprocessor/notify/notifier.go", textwrap.dedent("""\
    package notify

    import (
        "bytes"
        "context"
        "encoding/json"
        "net/http"
        "net/url"
        "time"

        "go.uber.org/zap"

        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor"
        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/statestore"
    )

    type Notifier struct {
        cfg alertsprocessor.NotifierConfig
        log *zap.Logger
        cli *http.Client
        endpointHost string
    }

    func New(cfg alertsprocessor.NotifierConfig, log *zap.Logger) *Notifier {
        var host string
        if u, err := url.Parse(cfg.URL); err == nil { host = u.Host }
        return &Notifier{
            cfg: cfg,
            log: log,
            cli: &http.Client{Timeout: cfg.Timeout},
            endpointHost: host,
        }
    }

    func (n *Notifier) Notify(ctx context.Context, trans []statestore.Transition) {
        if n.cfg.DisableSending || n.cfg.URL == "" || len(trans) == 0 { return }
        now := time.Now().UTC()
        alerts := make([]AMAlert, 0, len(trans))
        for _, t := range trans {
            status := "firing"
            endsAt := ""
            if t.To == "resolved" { status = "resolved"; endsAt = now.Format(time.RFC3339Nano) }
            alerts = append(alerts, AMAlert{
                Status: status,
                Labels: t.Labels,
                StartsAt: t.At.UTC().Format(time.RFC3339Nano),
                EndsAt: endsAt,
            })
        }
        b, _ := json.Marshal(alerts)
        req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.URL, bytes.NewReader(b))
        if err != nil { n.log.Warn("notifier: build request failed", zap.Error(err)); return }
        req.Header.Set("Content-Type", "application/json")
        resp, err := n.cli.Do(req)
        if err != nil { n.log.Warn("notifier: send failed", zap.Error(err)); return }
        _ = resp.Body.Close()
    }
    """))

# output
write("processor/alertsprocessor/output/series.go", textwrap.dedent("""\
    package output

    import (
        "time"

        "go.opentelemetry.io/collector/pdata/pcommon"
        "go.opentelemetry.io/collector/pdata/pmetric"

        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/evaluation"
        "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/statestore"
    )

    type SeriesBuilder struct {
        commonRes map[string]string
    }

    func NewSeriesBuilder(external map[string]string) *SeriesBuilder {
        return &SeriesBuilder{commonRes: external}
    }

    func (b *SeriesBuilder) Build(results []evaluation.Result, trans []statestore.Transition, ts time.Time) pmetric.Metrics {
        md := pmetric.NewMetrics()
        rm := md.ResourceMetrics().AppendEmpty()
        for k, v := range b.commonRes { rm.Resource().Attributes().PutStr(k, v) }
        sm := rm.ScopeMetrics().AppendEmpty()
        metrics := sm.Metrics()

        // 1) state gauge
        mState := metrics.AppendEmpty()
        mState.SetName("otel_alert_state")
        mState.SetEmptyGauge()
        for _, r := range results {
            for _, inst := range r.Instances {
                if !inst.Active { continue }
                dp := mState.Gauge().DataPoints().AppendEmpty()
                dp.SetIntValue(1)
                dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
                for k, v := range inst.Labels { dp.Attributes().PutStr(k, v) }
                dp.Attributes().PutStr("rule_id", r.Rule.ID)
                dp.Attributes().PutStr("signal", r.Signal)
            }
        }

        // 2) transitions counter
        mTrans := metrics.AppendEmpty()
        mTrans.SetName("otel_alert_transitions_total")
        mTrans.SetEmptySum()
        mTrans.Sum().SetIsMonotonic(true)
        mTrans.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
        for _, t := range trans {
            dp := mTrans.Sum().DataPoints().AppendEmpty()
            dp.SetIntValue(1)
            dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
            dp.Attributes().PutStr("rule_id", t.RuleID)
            dp.Attributes().PutStr("from", t.From)
            dp.Attributes().PutStr("to", t.To)
            dp.Attributes().PutStr("signal", t.Signal)
            for k, v := range t.Labels { dp.Attributes().PutStr(k, v) }
        }

        return md
    }
    """))

# testdata + smoke test
write("processor/alertsprocessor/testdata/config.yaml", textwrap.dedent("""\
    processors:
      alertsprocessor:
        sliding_window:
          duration: 5s
          max_samples: 10000
          overflow_behavior: ring_buffer
        evaluation:
          interval: 5s
          timeout: 5s
          max_concurrent: 4
        statestore:
          sync_interval: 30s
          instance_id: test
          external_labels:
            cluster: test
            source: collector
        deduplication:
          fingerprint_algorithm: sha256
          fingerprint_labels: ["alertname","cluster"]
        stormcontrol:
          global:
            max_active_alerts: 100
            max_alerts_per_minute: 50
        cardinality:
          labels:
            max_labels_per_alert: 20
            max_label_value_length: 128
            max_total_label_size: 2048
          allowlist: ["alertname","severity","cluster","service"]
        notifier:
          url: http://localhost:9093/api/v2/alerts
          timeout: 2s
          initial_interval: 200ms
          max_interval: 5s
          max_batch_size: 32
    """))

write("processor/alertsprocessor/factory_test.go", textwrap.dedent("""\
    package alertsprocessor

    import (
        "testing"

        "github.com/stretchr/testify/require"
        "go.opentelemetry.io/collector/component"
    )

    func TestCreateDefaultConfig(t *testing.T) {
        c := createDefaultConfig()
        require.NotNil(t, c)
        cfg := c.(*Config)
        require.Equal(t, component.MustNewID(typeStr), cfg.ID())
        require.NoError(t, cfg.Validate())
    }
    """))

zip_path = "./alertsprocessor-repo-latest.zip"
with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED) as z:
    for root, _, files in os.walk(base):
        for f in files:
            full = os.path.join(root, f)
            arc = os.path.relpath(full, base)
            z.write(full, arc)

zip_path
