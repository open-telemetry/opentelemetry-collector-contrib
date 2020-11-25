package useragentprocessor

import (
	"context"

	"github.com/ua-parser/uap-go/uaparser"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type userAgentProcessor struct {
	config       *Config
	logger       *zap.Logger
	uaParser     *uaparser.Parser
	nextConsumer consumer.TracesConsumer
}

func (u *userAgentProcessor) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		if rs.IsNil() {
			continue
		}
		ilss := rss.At(i).InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			if ils.IsNil() {
				continue
			}
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if span.IsNil() {
					// Do not create empty spans just to add attributes
					continue
				}
				if attr, ok := span.Attributes().Get(u.config.UserAgentTag); ok {
					u.enrichSpanWithUserAgent(attr.StringVal(), span)
				}
			}
		}
	}
	return u.nextConsumer.ConsumeTraces(ctx, td)
}

func (u *userAgentProcessor) Start(context.Context, component.Host) error {
	return nil
}

func (u *userAgentProcessor) Shutdown(context.Context) error {
	return nil
}

func (u *userAgentProcessor) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: true}
}

func (u *userAgentProcessor) enrichSpanWithUserAgent(userAgent string, span pdata.Span) {
	c := u.uaParser.Parse(userAgent)
	span.Attributes().Insert("browserFamily", pdata.NewAttributeValueString(c.UserAgent.Family))
	span.Attributes().Insert("browserVersion", pdata.NewAttributeValueString(c.UserAgent.ToVersionString()))
	span.Attributes().Insert("deviceFamily", pdata.NewAttributeValueString(c.Device.Family))
	span.Attributes().Insert("deviceBrand", pdata.NewAttributeValueString(c.Device.Brand))
	span.Attributes().Insert("deviceModel", pdata.NewAttributeValueString(c.Device.Model))
	span.Attributes().Insert("osFamily", pdata.NewAttributeValueString(c.Os.Family))
	span.Attributes().Insert("osVersion", pdata.NewAttributeValueString(c.Os.ToVersionString()))
}

func newProcessor(logger *zap.Logger, sender consumer.TracesConsumer, cfg *Config) (component.TracesProcessor, error) {
	uap := &userAgentProcessor{
		logger:       logger,
		nextConsumer: sender,
		config:       cfg,
	}
	var err error
	//initialize user agent parser
	uap.uaParser, err = uaparser.New(cfg.UserAgentFilePath)
	if err != nil {
		return nil, err
	}
	return uap, nil
}
