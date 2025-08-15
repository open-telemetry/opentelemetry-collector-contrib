package alertsprocessor // import "github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor"

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
	"go.uber.org/zap"

	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/cardinality"
	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/evaluation"
	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/notify"
	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/output"
	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/slidingwindow"
	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/statestore"
	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/stormcontrol"
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

	ticker  *time.Ticker
	curTick time.Duration
	stopCh  chan struct{}
	wg      sync.WaitGroup
	active  int // current number of firing alert instances (kept locally via transitions)
}

func newProcessor(_ context.Context, set processor.Settings, cfg *Config, m consumer.Metrics, l consumer.Logs, t consumer.Traces) (*processorImpl, error) {
	// Map parent config -> subpackage configs (no parent import in children)
	w := slidingwindow.New(slidingwindow.Config{
		Duration:         cfg.SlidingWindow.Duration,
		MaxSamples:       cfg.SlidingWindow.MaxSamples,
		OverflowBehavior: cfg.SlidingWindow.OverflowBehavior,
	})

	e := evaluation.NewEngine(
		evaluation.Sources{
			Files:  evaluation.RuleFiles{Include: cfg.RuleFiles.Include},
			Inline: cfg.Rules,
		},
		set.Logger,
	)

	st := statestore.New(cfg.Statestore, set.Logger)

	cdl := cardinality.Config{
		Labels: cardinality.LabelsCfg{
			MaxLabelsPerAlert:   cfg.Cardinality.Labels.MaxLabelsPerAlert,
			MaxLabelValueLength: cfg.Cardinality.Labels.MaxLabelValueLength,
			MaxTotalLabelSize:   cfg.Cardinality.Labels.MaxTotalLabelSize,
		},
		Allowlist:     cfg.Cardinality.Allowlist,
		Blocklist:     cfg.Cardinality.Blocklist,
		HashIfExceeds: cfg.Cardinality.HashIfExceeds,
		HashAlgorithm: cfg.Cardinality.HashAlgorithm,
		Series: cardinality.SeriesCfg{
			MaxActiveSeries:  cfg.Cardinality.Series.MaxActiveSeries,
			MaxSeriesPerRule: cfg.Cardinality.Series.MaxSeriesPerRule,
		},
	}
	cdl.Enforcement.Mode = cfg.Cardinality.Enforcement.Mode
	cdl.Enforcement.OverflowAction = cfg.Cardinality.Enforcement.OverflowAction
	cd := cardinality.New(cdl)

	gv := stormcontrol.New(cfg.StormControl)

	nf := notify.New(notify.Config{
		URL:             cfg.Notifier.URL,
		Timeout:         cfg.Notifier.Timeout,
		InitialInterval: cfg.Notifier.InitialInterval,
		MaxInterval:     cfg.Notifier.MaxInterval,
		MaxBatchSize:    cfg.Notifier.MaxBatchSize,
		DisableSending:  cfg.Notifier.DisableSending,
	}, set.Logger)

	ob := output.NewSeriesBuilder(cfg.Statestore.ExternalLabels)

	return &processorImpl{
		cfg:    cfg,
		set:    set,
		nextM:  m,
		nextL:  l,
		nextT:  t,
		win:    w,
		eval:   e,
		store:  st,
		card:   cd,
		gov:    gv,
		notif:  nf,
		out:    ob,
		stopCh: make(chan struct{}),
	}, nil
}

func (p *processorImpl) Start(ctx context.Context, _ component.Host) error {
	// Sliding window size warning (requested behavior)
	if p.cfg.SlidingWindow.Duration > 15*time.Second {
		p.set.Logger.Warn(
			"Large sliding_window.duration increases CPU and memory usage; consider keeping it small",
			zap.Duration("duration", p.cfg.SlidingWindow.Duration),
		)
	}

	// Initialize ticker from evaluation interval (fallback to 1s if misconfigured).
	p.curTick = p.cfg.Evaluation.Interval
	if p.curTick <= 0 {
		p.curTick = 1 * time.Second
	}
	p.ticker = time.NewTicker(p.curTick)

	p.wg.Add(1)
	go p.loop(ctx)
	return nil
}

func (p *processorImpl) Shutdown(context.Context) error {
	close(p.stopCh)
	if p.ticker != nil {
		p.ticker.Stop()
	}
	p.wg.Wait()
	return nil
}

func (p *processorImpl) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	p.win.IngestMetrics(md)
	return md, nil
}

func (p *processorImpl) processLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	p.win.IngestLogs(ld)
	return ld, nil
}

func (p *processorImpl) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	p.win.IngestTraces(td)
	return td, nil
}

func (p *processorImpl) loop(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-p.stopCh:
			return
		case <-p.ticker.C:
			p.evaluate(ctx, time.Now())
		case <-ctx.Done():
			return
		}
	}
}

func (p *processorImpl) evaluate(ctx context.Context, ts time.Time) {
	start := time.Now()

	// Snapshot the window.
	mets := p.win.SnapshotMetrics()
	logs := p.win.SnapshotLogs()
	trcs := p.win.SnapshotTraces()

	// Evaluate rules for all signals.
	var results []evaluation.Result
	results = append(results, p.eval.RunMetrics(mets, ts)...)
	results = append(results, p.eval.RunLogs(logs, ts)...)
	results = append(results, p.eval.RunTraces(trcs, ts)...)

	// Cardinality controls on labels
	for i := range results {
		results[i] = p.card.FilterResult(results[i])
	}

	// Apply to state; derive transitions.
	transitions := p.store.Apply(results, ts)

	// Maintain local 'active' count via transitions, so we don't depend on store internals.
	// We assume Transition.To is one of: "pending", "firing", "resolved".
	for _, tr := range transitions {
		switch tr.To {
		case "firing":
			p.active++
		case "resolved":
			if p.active > 0 {
				p.active--
			}
		}
	}

	// Emit synthetic metrics
	md := p.out.Build(results, transitions, ts)
	if md.DataPointCount() > 0 && p.nextM != nil {
		_ = p.nextM.ConsumeMetrics(ctx, md)
	}

	// Notify
	if p.notif != nil {
		p.notif.Notify(ctx, transitions)
	}

	// Self-telemetry
	p.emitEvalDuration(ctx, time.Since(start), ts)

	// Governor: adapt the next tick based on current activity and transitions-per-minute.
	if p.gov != nil && p.ticker != nil {
		apm := 0.0
		if p.curTick > 0 {
			apm = float64(len(transitions)) * 60.0 / p.curTick.Seconds()
		}
		next := p.gov.Update(p.active, apm)
		if next <= 0 {
			next = p.curTick // guard against zero/negative from misconfig
		}
		if next != p.curTick {
			p.curTick = next
			p.ticker.Reset(p.curTick)
		}
	}
}

func (p *processorImpl) emitEvalDuration(ctx context.Context, d time.Duration, ts time.Time) {
	if p.nextM == nil {
		return
	}
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
