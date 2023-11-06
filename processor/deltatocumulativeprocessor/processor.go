package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"
import (
	"context"
	"github.com/hashicorp/go-multierror"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"sync"
	"time"
)

type deltaToCumulativeProcessor struct {
	logger *zap.Logger

	// timer informs the processor send the metrics
	timer *time.Timer

	accumulator     accumulator
	metricsConsumer consumer.Metrics
	exportCtx       context.Context
	shutdownC       chan struct{}
	goroutines      sync.WaitGroup
	config          *Config
	telemetry       *telemetry
}

func newDeltaToCumulativeProcessor(ctx context.Context, config *Config, nextConsumer consumer.Metrics, settings *component.TelemetrySettings, useOtelForMetrics bool) *deltaToCumulativeProcessor {
	cl := client.FromContext(ctx)
	exportCtx := client.NewContext(context.Background(), cl)
	metrics, _ := newTelemetry(ctx, settings, config.Level, useOtelForMetrics)
	return &deltaToCumulativeProcessor{
		logger:          settings.Logger,
		metricsConsumer: nextConsumer,
		exportCtx:       exportCtx,
		shutdownC:       make(chan struct{}, 1),
		accumulator:     newAccumulator(settings.Logger, config.MaxStaleness, config.IdentifyMode, metrics),
		config:          config,
		telemetry:       metrics,
	}
}

func (p *deltaToCumulativeProcessor) Start(ctx context.Context, host component.Host) error {

	p.goroutines.Add(1)
	go p.start()
	return nil
}

func (p *deltaToCumulativeProcessor) start() {
	defer p.goroutines.Done()
	p.timer = time.NewTimer(p.config.SendInterval)
	var timerCh <-chan time.Time
	timerCh = p.timer.C

	for {
		select {
		case <-p.shutdownC:
			p.sendMetrics(p.exportCtx)
			return
		case <-timerCh:
			p.sendMetrics(p.exportCtx)
			if p.timer != nil {
				p.timer.Reset(p.config.SendInterval)
			}
		}
	}
}

func (p *deltaToCumulativeProcessor) Shutdown(ctx context.Context) error {
	close(p.shutdownC)

	// Wait until all goroutines are done.
	p.goroutines.Wait()
	return nil
}

func (p *deltaToCumulativeProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *deltaToCumulativeProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	n := 0
	rmetrics := md.ResourceMetrics()
	for i := 0; i < rmetrics.Len(); i++ {
		resourceMetrics := rmetrics.At(i)
		n += p.accumulator.Accumulate(resourceMetrics)
	}

	return nil
}

func (p *deltaToCumulativeProcessor) sendMetrics(ctx context.Context) error {
	var result error
	//now := pcommon.NewTimestampFromTime(time.Now())
	metrics, attrs := p.accumulator.Collect()

	for i := range attrs {
		metric := metrics[i]
		attributes := attrs[i]

		outputMetrics := pmetric.NewMetrics()
		resourceMetrics := outputMetrics.ResourceMetrics().AppendEmpty()
		attributes.CopyTo(resourceMetrics.Resource().Attributes())

		metric.CopyTo(resourceMetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())
		err := p.metricsConsumer.ConsumeMetrics(ctx, outputMetrics)
		multierror.Append(result, err)
	}
	return result
}
