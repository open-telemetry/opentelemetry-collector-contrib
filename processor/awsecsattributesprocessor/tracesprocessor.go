package awsecsattributesprocessor

import (
	"context"
	"fmt"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor/internal/metadata"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

var (
	traceIdReg = regexp.MustCompile(`[a-z0-9]{64}`)
)

// tracesDataProcessor implements DataProcessor for traces
type tracesDataProcessor struct {
	next consumer.Traces
}

var _ DataProcessor[ptrace.Traces] = (*tracesDataProcessor)(nil)

func (tdp *tracesDataProcessor) Process(ctx context.Context, logger *zap.Logger, cfg *Config, getMeta MetadataGetter, td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rtrace := td.ResourceSpans().At(i)
		containerID := getTraceContainerId(&rtrace, cfg.ContainerID.Sources...)
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

			rtrace.Resource().Attributes().
				PutStr(k, val)
		}
	}
	return nil
}

func (tdp *tracesDataProcessor) Forward(ctx context.Context, td ptrace.Traces) error {
	return tdp.next.ConsumeTraces(ctx, td)
}

// tracesProcessor wraps processorBase to implement processor.Traces interface
type tracesProcessor struct {
	*processorBase[ptrace.Traces, *tracesDataProcessor]
}

func (tp *tracesProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return tp.Consume(ctx, td)
}

// setNextConsumer sets the next consumer (used for testing)
func (tp *tracesProcessor) setNextConsumer(next consumer.Traces) {
	tp.dataProcessor.next = next
}

func newTracesProcessor(
	ctx context.Context,
	logger *zap.Logger,
	cfg *Config,
	next consumer.Traces,
	endpoints endpointsFn,
	containerdata containerDataFn,
) *tracesProcessor {
	dataProcessor := &tracesDataProcessor{next: next}
	base := newProcessorBase(ctx, logger, cfg, dataProcessor, endpoints, containerdata)
	return &tracesProcessor{processorBase: base}
}

func getTraceContainerId(rtrace *ptrace.ResourceSpans, sources ...string) string {
	var id string
	for _, s := range sources {
		if v, ok := rtrace.Resource().Attributes().Get(s); ok {
			id = v.AsString()
			break
		}
	}

	// strip any unneeded values for eg. file extension
	id = traceIdReg.FindString(id)
	return id
}
