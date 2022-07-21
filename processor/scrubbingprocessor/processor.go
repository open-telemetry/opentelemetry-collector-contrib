package scrubbingprocessor

import (
	"context"
	"regexp"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type scrubbingProcessor struct {
	logger *zap.Logger
	config *Config
}

func newScrubbingProcessorProcessor(logger *zap.Logger, cfg *Config) (*scrubbingProcessor, error) {
	return &scrubbingProcessor{
		logger: logger,
		config: cfg,
	}, nil
}

func (sp *scrubbingProcessor) ProcessLogs(ctx context.Context, logs plog.Logs) (plog.Logs, error) {
	if sp.config.Masking != nil {
		sp.applyMasking(logs)
	}

	return logs, nil
}

func (sp *scrubbingProcessor) applyMasking(ld plog.Logs) {

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resLogs := ld.ResourceLogs().At(i)
		for k := 0; k < resLogs.ScopeLogs().Len(); k++ {
			scopedLog := resLogs.ScopeLogs().At(k)
			for z := 0; z < scopedLog.LogRecords().Len(); z++ {
				log := scopedLog.LogRecords().At(z)
				for _, setting := range sp.config.Masking {
					regexp := regexp.MustCompile(setting.Regexp)
					log.Body().SetStringVal(regexp.ReplaceAllString(log.Body().AsString(), setting.Placeholder))
				}
			}
		}
	}

}
