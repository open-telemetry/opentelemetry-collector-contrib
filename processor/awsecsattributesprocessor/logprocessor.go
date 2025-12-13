package awsecsattributesprocessor

import (
	"context"
	"fmt"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor/internal/metadata"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

var (
	idReg = regexp.MustCompile(`[a-z0-9]{64}`)
)

// logsDataProcessor implements DataProcessor for logs
type logsDataProcessor struct {
	next consumer.Logs
}

var _ DataProcessor[plog.Logs] = (*logsDataProcessor)(nil)

func (ldp *logsDataProcessor) Process(ctx context.Context, logger *zap.Logger, cfg *Config, getMeta MetadataGetter, ld plog.Logs) error {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rlog := ld.ResourceLogs().At(i)
		containerID := getContainerId(&rlog, cfg.ContainerID.Sources...)
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

			rlog.Resource().Attributes().
				PutStr(k, val)
		}
	}
	return nil
}

func (ldp *logsDataProcessor) Forward(ctx context.Context, ld plog.Logs) error {
	return ldp.next.ConsumeLogs(ctx, ld)
}

// logsProcessor wraps processorBase to implement processor.Logs interface
type logsProcessor struct {
	*processorBase[plog.Logs, *logsDataProcessor]
}

func (lp *logsProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return lp.Consume(ctx, ld)
}

// setNextConsumer sets the next consumer (used for testing)
func (lp *logsProcessor) setNextConsumer(next consumer.Logs) {
	lp.dataProcessor.next = next
}

func newLogsProcessor(
	ctx context.Context,
	logger *zap.Logger,
	cfg *Config,
	next consumer.Logs,
	endpoints endpointsFn,
	containerdata containerDataFn,
) *logsProcessor {
	dataProcessor := &logsDataProcessor{next: next}
	base := newProcessorBase(ctx, logger, cfg, dataProcessor, endpoints, containerdata)
	return &logsProcessor{processorBase: base}
}
