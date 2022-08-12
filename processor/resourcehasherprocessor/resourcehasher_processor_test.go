// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resourcehasherprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

var (
	cfg = &Config{
		MaximumCacheSize:     100,
		MaximumCacheEntryAge: time.Duration(30 * float64(time.Second)),
	}
)

func TestResourceHasherProcessorClearAttributesOnRepeatedResource(t *testing.T) {
	logger := zaptest.NewLogger(t)

	factory := NewFactory()

	ttn := new(consumertest.TracesSink)

	settings := componenttest.NewNopProcessorCreateSettings()
	settings.TelemetrySettings.Logger = logger

	rtp, err := factory.CreateTracesProcessor(context.Background(), settings, cfg, ttn)
	require.NoError(t, err)

	sourceTraceData1 := generateTraceData(map[string]string{
		"cloud.availability_zone": "zone-1",
		"cloud.platform":          "aws_ecs",
	})
	err = rtp.ConsumeTraces(context.Background(), sourceTraceData1)
	require.NoError(t, err)

	sourceTraceData2 := generateTraceData(map[string]string{
		"cloud.availability_zone": "zone-1",
		"cloud.platform":          "aws_ecs",
	})
	err = rtp.ConsumeTraces(context.Background(), sourceTraceData2)

	require.NoError(t, err)
	traces := ttn.AllTraces()
	require.Len(t, traces, 2)

	// We have not seen this resource before, so we keep all attributes
	attributes1 := traces[0].ResourceSpans().At(0).Resource().Attributes()
	require.Equal(t, attributes1.Len(), 3)
	// TODO Check we still have the `cloud.availability_zone` attribute
	// TODO Check we have the `lumigo.resource.hash` attribute
	v1a, _ := attributes1.Get("cloud.availability_zone")
	require.Equal(t, v1a.StringVal(), "zone-1")
	v1b, _ := attributes1.Get("cloud.platform")
	require.Equal(t, v1b.StringVal(), "aws_ecs")
	v1c, _ := attributes1.Get("lumigo.resource.hash")
	require.Equal(t, v1c.StringVal(), "5318838988437335641")

	// We have seen this resource before, so we keep only the hash
	attributes2 := traces[1].ResourceSpans().At(0).Resource().Attributes()
	require.Equal(t, attributes2.Len(), 1)

	v2, _ := attributes2.Get("lumigo.resource.hash")
	require.Equal(t, v1c, v2)
}

func TestResourceHasherProcessorIgnoresSorting(t *testing.T) {
	logger := zaptest.NewLogger(t)

	factory := NewFactory()

	ttn := new(consumertest.TracesSink)

	settings := componenttest.NewNopProcessorCreateSettings()
	settings.TelemetrySettings.Logger = logger

	rtp, err := factory.CreateTracesProcessor(context.Background(), settings, cfg, ttn)
	require.NoError(t, err)

	sourceTraceData1 := generateTraceData(map[string]string{
		"cloud.availability_zone": "zone-1",
		"cloud.platform":          "aws_ecs",
	})
	err = rtp.ConsumeTraces(context.Background(), sourceTraceData1)
	require.NoError(t, err)

	sourceTraceData2 := generateTraceData(map[string]string{
		"cloud.platform":          "aws_ecs",
		"cloud.availability_zone": "zone-1",
	})
	err = rtp.ConsumeTraces(context.Background(), sourceTraceData2)

	require.NoError(t, err)
	traces := ttn.AllTraces()
	require.Len(t, traces, 2)

	// We have not seen this resource before, so we keep all attributes
	attributes1 := traces[0].ResourceSpans().At(0).Resource().Attributes()
	require.Equal(t, attributes1.Len(), 3)
	// TODO Check we still have the `cloud.availability_zone` attribute
	// TODO Check we have the `lumigo.resource.hash` attribute
	v1a, _ := attributes1.Get("cloud.availability_zone")
	require.Equal(t, v1a.StringVal(), "zone-1")
	v1b, _ := attributes1.Get("cloud.platform")
	require.Equal(t, v1b.StringVal(), "aws_ecs")
	v1c, _ := attributes1.Get("lumigo.resource.hash")
	require.Equal(t, v1c.StringVal(), "5318838988437335641")

	// We have seen this resource before, so we keep only the hash
	attributes2 := traces[1].ResourceSpans().At(0).Resource().Attributes()
	require.Equal(t, attributes2.Len(), 1)

	v2, _ := attributes2.Get("lumigo.resource.hash")
	require.Equal(t, v1c, v2)
}

func TestResourceHasherProcessorIgnoresLumigoHashAttribute(t *testing.T) {
	logger := zaptest.NewLogger(t)

	factory := NewFactory()

	ttn := new(consumertest.TracesSink)

	settings := componenttest.NewNopProcessorCreateSettings()
	settings.TelemetrySettings.Logger = logger

	rtp, err := factory.CreateTracesProcessor(context.Background(), settings, cfg, ttn)
	require.NoError(t, err)

	sourceTraceData1 := generateTraceData(map[string]string{
		"cloud.availability_zone": "zone-1",
		"cloud.platform":          "aws_ecs",
		"lumigo.resource.hash":    "1",
	})
	err = rtp.ConsumeTraces(context.Background(), sourceTraceData1)
	require.NoError(t, err)

	sourceTraceData2 := generateTraceData(map[string]string{
		"lumigo.resource.hash":    "42",
		"cloud.availability_zone": "zone-1",
		"cloud.platform":          "aws_ecs",
	})
	err = rtp.ConsumeTraces(context.Background(), sourceTraceData2)

	require.NoError(t, err)
	traces := ttn.AllTraces()
	require.Len(t, traces, 2)

	// We have not seen this resource before, so we keep all attributes
	attributes1 := traces[0].ResourceSpans().At(0).Resource().Attributes()
	require.Equal(t, attributes1.Len(), 3)
	// TODO Check we still have the `cloud.availability_zone` attribute
	// TODO Check we have the `lumigo.resource.hash` attribute
	v1a, _ := attributes1.Get("cloud.availability_zone")
	require.Equal(t, v1a.StringVal(), "zone-1")
	v1b, _ := attributes1.Get("cloud.platform")
	require.Equal(t, v1b.StringVal(), "aws_ecs")
	v1c, _ := attributes1.Get("lumigo.resource.hash")
	require.Equal(t, v1c.StringVal(), "5318838988437335641")

	// We have seen this resource before, so we keep only the hash
	attributes2 := traces[1].ResourceSpans().At(0).Resource().Attributes()
	require.Equal(t, attributes2.Len(), 1)

	v2, _ := attributes2.Get("lumigo.resource.hash")
	require.Equal(t, v1c, v2)
}

func TestResourceHasherProcessorSendsFullResourceAfterElapsedWaitTime(t *testing.T) {
	// Test that, if we wait more that the max elapse time between full-resource sends,
	// we re-send the full resource
}

func generateTraceData(attributes map[string]string) ptrace.Traces {
	td := testdata.GenerateTracesOneSpanNoResource()
	if attributes == nil {
		return td
	}
	resource := td.ResourceSpans().At(0).Resource()
	for k, v := range attributes {
		resource.Attributes().InsertString(k, v)
	}
	resource.Attributes().Sort()
	return td
}
