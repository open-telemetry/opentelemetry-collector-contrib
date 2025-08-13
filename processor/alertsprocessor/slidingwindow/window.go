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
