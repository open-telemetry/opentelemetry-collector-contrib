// Copyright OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuremonitorexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

var (
	defaultConfig = createDefaultConfig().(*Config)
)

// Tests the export onTraceData callback with no Spans
func TestExporterTraceDataCallbackNoSpans(t *testing.T) {
	mockTransportChannel := getMockTransportChannel()
	exporter := getExporter(defaultConfig, mockTransportChannel)

	traces := pdata.NewTraces()

	droppedSpans, err := exporter.onTraceData(context.Background(), traces)
	assert.Nil(t, err)
	assert.Equal(t, 0, droppedSpans)

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 0)
}

// Tests the export onTraceData callback with a single Span
func TestExporterTraceDataCallbackSingleSpan(t *testing.T) {
	mockTransportChannel := getMockTransportChannel()
	exporter := getExporter(defaultConfig, mockTransportChannel)

	// re-use some test generation method(s) from trace_to_envelope_test
	resource := getResource()
	instrumentationLibrary := getInstrumentationLibrary()
	span := getDefaultHTTPServerSpan()

	traces := pdata.NewTraces()
	traces.ResourceSpans().Resize(1)
	rs := traces.ResourceSpans().At(0)
	r := rs.Resource()
	resource.CopyTo(r)
	rs.InstrumentationLibrarySpans().Resize(1)
	ilss := rs.InstrumentationLibrarySpans().At(0)
	instrumentationLibrary.CopyTo(ilss.InstrumentationLibrary())
	ilss.Spans().Resize(1)
	span.CopyTo(ilss.Spans().At(0))

	droppedSpans, err := exporter.onTraceData(context.Background(), traces)
	assert.Nil(t, err)
	assert.Equal(t, 0, droppedSpans)

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 1)
}

// Tests the export onTraceData callback with a single Span that fails to produce an envelope
func TestExporterTraceDataCallbackSingleSpanNoEnvelope(t *testing.T) {
	mockTransportChannel := getMockTransportChannel()
	exporter := getExporter(defaultConfig, mockTransportChannel)

	// re-use some test generation method(s) from trace_to_envelope_test
	resource := getResource()
	instrumentationLibrary := getInstrumentationLibrary()
	span := getDefaultInternalSpan()

	// Make this a FaaS span, which will trigger an error, because conversion
	// of them is currently not supported.
	span.Attributes().InsertString(conventions.AttributeFaaSTrigger, "http")

	traces := pdata.NewTraces()
	traces.ResourceSpans().Resize(1)
	rs := traces.ResourceSpans().At(0)
	r := rs.Resource()
	resource.CopyTo(r)
	rs.InstrumentationLibrarySpans().Resize(1)
	ilss := rs.InstrumentationLibrarySpans().At(0)
	instrumentationLibrary.CopyTo(ilss.InstrumentationLibrary())
	ilss.Spans().Resize(1)
	span.CopyTo(ilss.Spans().At(0))

	droppedSpans, err := exporter.onTraceData(context.Background(), traces)
	assert.NotNil(t, err)
	assert.True(t, consumererror.IsPermanent(err), "error should be permanent")
	assert.Equal(t, 1, droppedSpans)

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 0)
}

func getMockTransportChannel() *mockTransportChannel {
	transportChannelMock := mockTransportChannel{}
	transportChannelMock.On("Send", mock.Anything)
	return &transportChannelMock
}

func getExporter(config *Config, transportChannel transportChannel) *traceExporter {
	return &traceExporter{
		config,
		transportChannel,
		zap.NewNop(),
	}
}
