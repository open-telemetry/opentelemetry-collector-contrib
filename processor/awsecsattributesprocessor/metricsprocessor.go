package awsecsattributesprocessor

import (
	"context"
	"fmt"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor/internal/metadata"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var (
	metricIdReg = regexp.MustCompile(`[a-z0-9]{64}`)
)

// metricsDataProcessor implements DataProcessor for metrics
type metricsDataProcessor struct {
	next consumer.Metrics
}

var _ DataProcessor[pmetric.Metrics] = (*metricsDataProcessor)(nil)

func (mdp *metricsDataProcessor) Process(ctx context.Context, logger *zap.Logger, cfg *Config, getMeta MetadataGetter, md pmetric.Metrics) error {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rmetric := md.ResourceMetrics().At(i)
		containerID := getMetricContainerId(&rmetric, cfg.ContainerID.Sources...)
		logger.Debug("processing",
			zap.String("container.id", containerID),
			zap.String("processor", metadata.Type.String()))

		meta, err := getMeta(containerID)
		if err != nil {
			logger.Debug("unable to retrieve metadata",
				zap.String("container.id", containerID),
				zap.String("processor", metadata.Type.String()),
				zap.Error(err))

			continue
		}

		// flatten the data
		flattened := meta.Flat()

		for k, v := range flattened {
			val := fmt.Sprintf("%v", v)
			// if not allowed or empty value, skip
			if !cfg.allowAttr(k) || val == "" {
				continue
			}

			rmetric.Resource().Attributes().
				PutStr(k, val)
		}
	}
	return nil
}

func (mdp *metricsDataProcessor) Forward(ctx context.Context, md pmetric.Metrics) error {
	return mdp.next.ConsumeMetrics(ctx, md)
}

// metricsProcessor wraps processorBase to implement processor.Metrics interface
type metricsProcessor struct {
	*processorBase[pmetric.Metrics, *metricsDataProcessor]
}

func (mp *metricsProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return mp.Consume(ctx, md)
}

// setNextConsumer sets the next consumer (used for testing)
func (mp *metricsProcessor) setNextConsumer(next consumer.Metrics) {
	mp.dataProcessor.next = next
}

func newMetricsProcessor(
	ctx context.Context,
	logger *zap.Logger,
	cfg *Config,
	next consumer.Metrics,
	endpoints endpointsFn,
	containerdata containerDataFn,
) *metricsProcessor {
	dataProcessor := &metricsDataProcessor{next: next}
	base := newProcessorBase(ctx, logger, cfg, dataProcessor, endpoints, containerdata)
	return &metricsProcessor{processorBase: base}
}

func getMetricContainerId(rmetric *pmetric.ResourceMetrics, sources ...string) string {
	var id string
	for _, s := range sources {
		if v, ok := rmetric.Resource().Attributes().Get(s); ok {
			id = v.AsString()
			break
		}
	}

	// strip any unneeded values for eg. file extension
	id = metricIdReg.FindString(id)
	return id
}
