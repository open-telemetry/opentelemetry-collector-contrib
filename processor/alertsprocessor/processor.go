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
	e := evaluation.NewEngine(evaluation.Sources{Files: evaluation.RuleFiles{Include: cfg.RuleFiles.Include}, Inline: cfg.Rules}, set.Logger)
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
