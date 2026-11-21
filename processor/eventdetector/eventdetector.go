// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eventdetector // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/eventdetector"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/eventdetector/internal/metadata"
)

// signalstateClient is the slice of the signalstate extension the detector
// uses, declared as an interface so tests can substitute it.
type signalstateClient interface {
	Open(ctx context.Context, name string, ttl time.Duration)
	IsOpen(ctx context.Context, name string) bool
	ExpiresAt(ctx context.Context, name string) time.Time
}

type logRule struct {
	cfg   LogRule
	conds *ottl.ConditionSequence[*ottllog.TransformContext]
}

type metricRule struct {
	cfg   MetricRule
	conds *ottl.ConditionSequence[*ottldatapoint.TransformContext]
}

// detectorCore holds rules and the shared-state client. Each configured
// pipeline gets its own core instance; cross-pipeline coherence comes from
// the shared signalstate extension, which is the memory here, not the
// processor.
type detectorCore struct {
	cfg *Config
	log *zap.Logger

	logRules    []logRule
	metricRules []metricRule

	matches otelmetric.Int64Counter
	events  otelmetric.Int64Counter

	signalState signalstateClient
	now         func() time.Time
}

type detectorLogs struct {
	core *detectorCore
	next consumer.Logs
}

func (l *detectorLogs) Start(ctx context.Context, host component.Host) error {
	return l.core.Start(ctx, host)
}

func (l *detectorLogs) Shutdown(ctx context.Context) error { return l.core.Shutdown(ctx) }

func (l *detectorLogs) Capabilities() consumer.Capabilities {
	return l.core.Capabilities()
}

func (l *detectorLogs) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	l.core.processLogs(ctx, ld)
	return l.next.ConsumeLogs(ctx, ld)
}

type detectorMetrics struct {
	core *detectorCore
	next consumer.Metrics
}

func newCore(cfg *Config, set processor.Settings) (*detectorCore, error) {
	tb, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	d := &detectorCore{
		cfg:      cfg,
		log:      set.Logger,
		matches:  tb.OtelcolEventdetectorMatches,
		events:   tb.OtelcolEventdetectorEvents,
		now:      time.Now,
	}

	for i, rc := range cfg.LogRules {
		seq, err := filterottl.NewBoolExprForLog(rc.Conditions, filterottl.StandardLogFuncs(), ottl.PropagateError, set.TelemetrySettings)
		if err != nil {
			return nil, wrapRuleErr("log", i, rc.Name, err)
		}
		d.logRules = append(d.logRules, logRule{cfg: rc, conds: seq})
	}
	for i, rc := range cfg.MetricRules {
		seq, err := filterottl.NewBoolExprForDataPoint(rc.Conditions, filterottl.StandardDataPointFuncs(), ottl.PropagateError, set.TelemetrySettings)
		if err != nil {
			return nil, wrapRuleErr("metric", i, rc.Name, err)
		}
		d.metricRules = append(d.metricRules, metricRule{cfg: rc, conds: seq})
	}
	return d, nil
}

func wrapRuleErr(kind string, idx int, name string, err error) error {
	return err
}

func (d *detectorCore) Start(_ context.Context, host component.Host) error {
	if d.cfg.SignalstateExtension == nil {
		// fail toward visibility: no windows open, everything passes through
		d.log.Warn("eventdetector: signalstate_extension unset; windows will not open")
		return nil
	}
	exts := host.GetExtensions()
	if ext, ok := exts[*d.cfg.SignalstateExtension]; ok && ext != nil {
		if ss, ok := ext.(signalstateClient); ok {
			d.signalState = ss
			return nil
		}
		d.log.Warn("eventdetector: configured extension does not implement signalstateClient; windows will not open",
			zap.String("extension_id", d.cfg.SignalstateExtension.String()))
		return nil
	}
	d.log.Warn("eventdetector: signalstate extension not found; windows will not open",
		zap.String("extension_id", d.cfg.SignalstateExtension.String()))
	return nil
}

func (d *detectorCore) Shutdown(_ context.Context) error { return nil }

func (d *detectorCore) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// openWindow applies detector-level hysteresis: if the window closed inside
// the rule's cooldown, re-opening is refused until the cooldown has passed.
// Evaluation errors never open windows (fail toward calm), per the
// PropagateError mode scoping evaluation errors to the single record.
func (d *detectorCore) openWindow(ctx context.Context, name string, ttl, cooldown time.Duration, rule string) {
	if d.signalState == nil {
		return
	}
	now := d.now()
	if exp := d.signalState.ExpiresAt(ctx, name); !exp.IsZero() {
		if now.Before(exp) {
			// already open: extend
			d.signalState.Open(ctx, name, ttl)
			d.matches.Add(ctx, 1, otelmetric.WithAttributes(
				attribute.String("rule_name", rule),
				attribute.String("window_name", name)))
			return
		}
		if now.Sub(exp) < cooldown {
			// inside cooldown: refuse to re-open (anti-flap)
			return
		}
	}
	d.signalState.Open(ctx, name, ttl)
	d.matches.Add(ctx, 1, otelmetric.WithAttributes(
		attribute.String("rule_name", rule),
		attribute.String("window_name", name)))
}

func (d *detectorCore) processLogs(ctx context.Context, ld plog.Logs) {
	d.events.Add(ctx, int64(ld.LogRecordCount()))

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rm := rls.At(i)
		for j := 0; j < rls.At(i).ScopeLogs().Len(); j++ {
			sl := rls.At(i).ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				rec := sl.LogRecords().At(k)
				tCtx := ottllog.NewTransformContext(rm, sl, rec)
				for ri := range d.logRules {
					r := &d.logRules[ri]
					ok, err := r.conds.Eval(ctx, tCtx)
					if err != nil {
						d.log.Debug("eventdetector: log condition evaluation failed",
							zap.String("rule", r.cfg.Name), zap.Error(err))
						continue
					}
					if ok {
						d.openWindow(ctx, r.cfg.Window, r.cfg.Ttl, r.cfg.effectiveCooldown(), r.cfg.Name)
					}
				}
				tCtx.Close()
			}
		}
	}
}

func (d *detectorCore) processMetrics(ctx context.Context, md pmetric.Metrics) {
	d.events.Add(ctx, datapointCount(md))

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				met := sm.Metrics().At(k)
				for _, tc := range datapointContexts(rm, sm, met) {
					for ri := range d.metricRules {
						r := &d.metricRules[ri]
						ok, err := r.conds.Eval(ctx, tc)
						if err != nil {
							d.log.Debug("eventdetector: metric condition evaluation failed",
								zap.String("rule", r.cfg.Name), zap.Error(err))
							continue
						}
						if ok {
							d.openWindow(ctx, r.cfg.Window, r.cfg.Ttl, r.cfg.effectiveCooldown(), r.cfg.Name)
						}
					}
					tc.Close()
				}
			}
		}
	}
}

func (m *detectorMetrics) Start(ctx context.Context, host component.Host) error {
	return m.core.Start(ctx, host)
}

func (m *detectorMetrics) Shutdown(ctx context.Context) error { return m.core.Shutdown(ctx) }

func (m *detectorMetrics) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	m.core.processMetrics(ctx, md)
	return m.next.ConsumeMetrics(ctx, md)
}

func (m *detectorMetrics) Capabilities() consumer.Capabilities {
	return m.core.Capabilities()
}

func datapointContexts(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, met pmetric.Metric) []*ottldatapoint.TransformContext {
	out := []*ottldatapoint.TransformContext{}
	switch met.Type() {
	case pmetric.MetricTypeGauge:
		for i := 0; i < met.Gauge().DataPoints().Len(); i++ {
			out = append(out, ottldatapoint.NewTransformContext(rm, sm, met, met.Gauge().DataPoints().At(i)))
		}
	case pmetric.MetricTypeSum:
		for i := 0; i < met.Sum().DataPoints().Len(); i++ {
			out = append(out, ottldatapoint.NewTransformContext(rm, sm, met, met.Sum().DataPoints().At(i)))
		}
	case pmetric.MetricTypeHistogram:
		for i := 0; i < met.Histogram().DataPoints().Len(); i++ {
			out = append(out, ottldatapoint.NewTransformContext(rm, sm, met, met.Histogram().DataPoints().At(i)))
		}
	}
	return out
}

func datapointCount(md pmetric.Metrics) int64 {
	var n int64
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		for j := 0; j < md.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			for k := 0; k < md.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
				n += int64(datapointContextCount(md.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)))
			}
		}
	}
	return n
}

func datapointContextCount(met pmetric.Metric) int {
	switch met.Type() {
	case pmetric.MetricTypeGauge:
		return met.Gauge().DataPoints().Len()
	case pmetric.MetricTypeSum:
		return met.Sum().DataPoints().Len()
	case pmetric.MetricTypeHistogram:
		return met.Histogram().DataPoints().Len()
	}
	return 0
}
