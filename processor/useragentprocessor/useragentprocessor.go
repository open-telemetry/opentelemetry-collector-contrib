package useragentprocessor

import (
	"context"
	"go.opentelemetry.io/collector/translator/conventions"

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
		if rs.IsNil() || rs.Resource().IsNil() {
			continue
		}
		if attr, ok := rs.Resource().Attributes().Get(conventions.AttributeHTTPUserAgent); ok {
			u.enrichResourceWithUserAgent(attr.StringVal(), rs.Resource())
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

func (u *userAgentProcessor) enrichResourceWithUserAgent(userAgent string, resource pdata.Resource) {
	c := u.uaParser.Parse(userAgent)
	resource.Attributes().Insert("browserFamily", pdata.NewAttributeValueString(c.UserAgent.Family))
	resource.Attributes().Insert("browserVersion", pdata.NewAttributeValueString(c.UserAgent.ToVersionString()))
	resource.Attributes().Insert("deviceFamily", pdata.NewAttributeValueString(c.Device.Family))
	resource.Attributes().Insert("deviceBrand", pdata.NewAttributeValueString(c.Device.Brand))
	resource.Attributes().Insert("deviceModel", pdata.NewAttributeValueString(c.Device.Model))
	resource.Attributes().Insert("osFamily", pdata.NewAttributeValueString(c.Os.Family))
	resource.Attributes().Insert("osVersion", pdata.NewAttributeValueString(c.Os.ToVersionString()))
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
