package scrubbingprocessor

import (
	"context"
	"regexp"

	"go.opentelemetry.io/collector/pdata/pcommon"
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

func (sp *scrubbingProcessor) ProcessLogs(_ context.Context, logs plog.Logs) (plog.Logs, error) {
	if sp.config.Masking != nil {
		sp.applyMasking(logs)
	}

	return logs, nil
}

func (sp *scrubbingProcessor) applyMasking(ld plog.Logs) {
	// masking in resource attributes
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceAttributes := ld.ResourceLogs().At(i).Resource().Attributes()
		for _, setting := range sp.config.Masking {
			rExp := regexp.MustCompile(setting.Regexp)

			if (setting.AttributeType == ResourceAttribute || setting.AttributeType == EmptyAttribute) && setting.AttributeKey == "" {
				resourceAttributes.Range(func(key string, attributeValue pcommon.Value) bool {
					attributeValue.SetStr(rExp.ReplaceAllString(attributeValue.AsString(), setting.Placeholder))
					return true
				})
			} else if (setting.AttributeType == ResourceAttribute || setting.AttributeType == EmptyAttribute) && setting.AttributeKey != "" {
				if attributeValue, ok := resourceAttributes.Get(setting.AttributeKey); ok {
					attributeValue.SetStr(rExp.ReplaceAllString(attributeValue.AsString(), setting.Placeholder))
				}
			}
		}
	}

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resLogs := ld.ResourceLogs().At(i)
		for k := 0; k < resLogs.ScopeLogs().Len(); k++ {
			scopedLog := resLogs.ScopeLogs().At(k)
			for z := 0; z < scopedLog.LogRecords().Len(); z++ {
				log := scopedLog.LogRecords().At(z)
				for _, setting := range sp.config.Masking {
					rExp := regexp.MustCompile(setting.Regexp)

					// masking in record attributes
					if (setting.AttributeType == RecordAttribute || setting.AttributeType == EmptyAttribute) && setting.AttributeKey == "" {
						log.Attributes().Range(func(key string, attributeValue pcommon.Value) bool {
							attributeValue.SetStr(rExp.ReplaceAllString(attributeValue.AsString(), setting.Placeholder))
							return true
						})
					} else if (setting.AttributeType == RecordAttribute || setting.AttributeType == EmptyAttribute) && setting.AttributeKey != "" {
						if attributeValue, ok := log.Attributes().Get(setting.AttributeKey); ok {
							attributeValue.SetStr(rExp.ReplaceAllString(attributeValue.AsString(), setting.Placeholder))
						}
					}

					// masking body
					switch log.Body().Type() {
					case pcommon.ValueTypeMap:
						log.Body().Map().Range(func(k string, v pcommon.Value) bool {
							v.SetStr(rExp.ReplaceAllString(v.AsString(), setting.Placeholder))
							return true
						})
					case pcommon.ValueTypeStr:
						log.Body().SetStr(rExp.ReplaceAllString(log.Body().AsString(), setting.Placeholder))
					}
				}
			}
		}
	}
}
