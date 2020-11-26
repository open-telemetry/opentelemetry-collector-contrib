package useragentprocessor

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"
	"path"
	"testing"
)

// Common structure for all the Tests
type testCase struct {
	name               string
	serviceName        string
	inputAttributes    map[string]pdata.AttributeValue
	expectedAttributes map[string]pdata.AttributeValue
}

func generateTraceData(serviceName, spanName string, attrMap map[string]pdata.AttributeValue) pdata.Traces {
	td := pdata.NewTraces()
	td.ResourceSpans().Resize(1)
	rs := td.ResourceSpans().At(0)
	if serviceName != "" {
		rs.Resource().InitEmpty()
		rs.Resource().Attributes().InitFromMap(attrMap)
		rs.Resource().Attributes().InsertString(conventions.AttributeServiceName, serviceName)
	}
	rs.InstrumentationLibrarySpans().Resize(1)
	ils := rs.InstrumentationLibrarySpans().At(0)
	ils.InitEmpty()
	spans := ils.Spans()
	spans.Resize(1)
	s := spans.At(0)
	s.InitEmpty()
	s.SetName(spanName)
	return td
}

func TestResourceAttributesProcessor(t *testing.T) {
	attrWithoutUserAgent := make(map[string]pdata.AttributeValue)

	attrWithUserAgent := make(map[string]pdata.AttributeValue)
	attrWithUserAgent[conventions.AttributeHTTPUserAgent] = pdata.NewAttributeValueString("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.111 Safari/537.36")

	attrWithUserAgentOutput := make(map[string]pdata.AttributeValue)
	attrWithUserAgentOutput[conventions.AttributeHTTPUserAgent] = pdata.NewAttributeValueString("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.111 Safari/537.36")
	attrWithUserAgentOutput["browserFamily"] = pdata.NewAttributeValueString("Chrome")
	attrWithUserAgentOutput["browserVersion"] = pdata.NewAttributeValueString("86.0.4240")
	attrWithUserAgentOutput["deviceFamily"] = pdata.NewAttributeValueString("Mac")
	attrWithUserAgentOutput["deviceBrand"] = pdata.NewAttributeValueString("Apple")
	attrWithUserAgentOutput["deviceModel"] = pdata.NewAttributeValueString("Mac")
	attrWithUserAgentOutput["osFamily"] = pdata.NewAttributeValueString("Mac OS X")
	attrWithUserAgentOutput["osVersion"] = pdata.NewAttributeValueString("10.15.7")

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.UserAgentFilePath = path.Join(".", "resources", "regexes.yaml")
	tp, err := factory.CreateTracesProcessor(context.Background(), component.ProcessorCreateParams{Logger: zap.NewNop()}, oCfg, consumertest.NewTracesNop())
	require.Nil(t, err)
	require.NotNil(t, tp)
	testCases := []testCase{
		{
			name:               "withoutUserAgentTag",
			serviceName:        "fooService",
			inputAttributes:    attrWithoutUserAgent,
			expectedAttributes: attrWithoutUserAgent,
		},
		{
			name:               "withUserAgentTag",
			serviceName:        "fooService",
			inputAttributes:    attrWithUserAgent,
			expectedAttributes: attrWithUserAgentOutput,
		},
	}
	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			td := generateTraceData(tt.serviceName, tt.name, tt.inputAttributes)
			err = tp.ConsumeTraces(context.Background(), td)
			assert.NoError(t, err)

			am := td.ResourceSpans().At(0).Resource().Attributes()
			outputAttr := make(map[string]pdata.AttributeValue)
			// this gets added in the generateTraceData method
			tt.expectedAttributes[conventions.AttributeServiceName] = pdata.NewAttributeValueString(tt.serviceName)

			am.ForEach(func(k string, v pdata.AttributeValue) {
				outputAttr[k] = v
			})
			assert.EqualValues(t, tt.expectedAttributes, outputAttr)
		})
	}

}
