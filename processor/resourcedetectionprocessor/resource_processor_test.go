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

package resourcedetectionprocessor

import (
	"context"
	"testing"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/env"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/gce"
)

type MockDetector struct {
	mock.Mock
}

func (p *MockDetector) Detect(ctx context.Context) (pdata.Resource, error) {
	args := p.Called()
	return args.Get(0).(pdata.Resource), args.Error(1)
}

func TestResourceProcessor(t *testing.T) {
	tests := []struct {
		name                string
		mutatesConsumedData bool
		override            bool
		sourceResource      pdata.Resource
		detectedResource    pdata.Resource
		wantResource        pdata.Resource
	}{
		{
			name:                "Resource is not overridden",
			mutatesConsumedData: true,
			override:            false,
			sourceResource: internal.NewResource(map[string]string{
				"type":           "original-type",
				"original-label": "original-value",
				"cloud.zone":     "original-zone",
			}),
			detectedResource: internal.NewResource(map[string]string{
				"cloud.zone":       "will-be-ignored",
				"k8s.cluster.name": "k8s-cluster",
				"host.name":        "k8s-node",
			}),
			wantResource: internal.NewResource(map[string]string{
				"type":             "original-type",
				"original-label":   "original-value",
				"cloud.zone":       "original-zone",
				"k8s.cluster.name": "k8s-cluster",
				"host.name":        "k8s-node",
			}),
		},
		{
			name:                "Resource is overridden",
			mutatesConsumedData: true,
			override:            true,
			sourceResource: internal.NewResource(map[string]string{
				"type":           "original-type",
				"original-label": "original-value",
				"cloud.zone":     "will-be-overridden",
			}),
			detectedResource: internal.NewResource(map[string]string{
				"cloud.zone":       "zone-1",
				"k8s.cluster.name": "k8s-cluster",
				"host.name":        "k8s-node",
			}),
			wantResource: internal.NewResource(map[string]string{
				"type":             "original-type",
				"original-label":   "original-value",
				"cloud.zone":       "zone-1",
				"k8s.cluster.name": "k8s-cluster",
				"host.name":        "k8s-node",
			}),
		},
		{
			name:                "Empty detected resource",
			mutatesConsumedData: false,
			sourceResource: internal.NewResource(map[string]string{
				"type":           "original-type",
				"original-label": "original-value",
				"cloud.zone":     "original-zone",
			}),
			detectedResource: internal.NewResource(map[string]string{}),
			wantResource: internal.NewResource(map[string]string{
				"type":           "original-type",
				"original-label": "original-value",
				"cloud.zone":     "original-zone",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantResource.Attributes().Sort()

			md1 := &MockDetector{}
			md1.On("Detect").Return(tt.detectedResource, nil)
			detectors := map[string]internal.Detector{"mock": md1}

			cfg := &Config{Override: tt.override, Detectors: []string{"mock"}}

			// Test trace consuner
			ttn := &exportertest.SinkTraceExporter{}
			rtp, err := newResourceTraceProcessor(context.Background(), ttn, cfg, detectors)
			require.NoError(t, err)
			assert.Equal(t, tt.mutatesConsumedData, rtp.GetCapabilities().MutatesConsumedData)

			td := pdata.NewTraces()
			td.ResourceSpans().Resize(1)
			tt.sourceResource.CopyTo(td.ResourceSpans().At(0).Resource())

			err = rtp.ConsumeTraces(context.Background(), td)
			require.NoError(t, err)
			got := ttn.AllTraces()[0].ResourceSpans().At(0).Resource()
			got.Attributes().Sort()
			assert.Equal(t, tt.wantResource, got)

			// Test metrics consumer
			tmn := &exportertest.SinkMetricsExporter{}
			rmp, err := newResourceMetricProcessor(context.Background(), tmn, cfg, detectors)
			require.NoError(t, err)
			assert.Equal(t, tt.mutatesConsumedData, rmp.GetCapabilities().MutatesConsumedData)

			// TODO create pdata.Metrics directly when this is no longer internal
			md := []consumerdata.MetricsData{{Resource: oCensusResource(tt.sourceResource)}}

			err = rmp.ConsumeMetrics(context.Background(), pdatautil.MetricsFromMetricsData(md))
			require.NoError(t, err)
			got = pdatautil.MetricsToInternalMetrics(tmn.AllMetrics()[0]).ResourceMetrics().At(0).Resource()
			got.Attributes().Sort()
			assert.Equal(t, tt.wantResource, got)
		})
	}
}

func oCensusResource(res pdata.Resource) *resourcepb.Resource {
	mp := make(map[string]string, res.Attributes().Len())
	res.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		mp[k] = v.StringVal()
	})

	return &resourcepb.Resource{Labels: mp}
}

func benchmarkConsumeTraces(b *testing.B, cfg *Config) {
	detectors := NewFactory().detectors
	sink := &exportertest.SinkTraceExporter{}

	processor, _ := newResourceTraceProcessor(context.Background(), sink, cfg, detectors)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// TODO generate real data using https://github.com/open-telemetry/opentelemetry-collector/pull/1062/files#diff-cf39274cfadf030e1797200cd5218d7bR116 when available
		processor.ConsumeTraces(context.Background(), pdata.NewTraces())
	}
}

func BenchmarkConsumeTracesDefault(b *testing.B) {
	cfg := NewFactory().CreateDefaultConfig()
	benchmarkConsumeTraces(b, cfg.(*Config))
}

func BenchmarkConsumeTracesAll(b *testing.B) {
	cfg := &Config{Override: true, Detectors: []string{env.TypeStr, gce.TypeStr}}
	benchmarkConsumeTraces(b, cfg)
}

func benchmarkConsumeMetrics(b *testing.B, cfg *Config) {
	detectors := NewFactory().detectors
	sink := &exportertest.SinkMetricsExporter{}

	processor, _ := newResourceMetricProcessor(context.Background(), sink, cfg, detectors)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// TODO generate real data using https://github.com/open-telemetry/opentelemetry-collector/pull/1062/files#diff-cf39274cfadf030e1797200cd5218d7bR221 when available
		processor.ConsumeMetrics(context.Background(), pdatautil.MetricsFromMetricsData(nil))
	}
}

func BenchmarkConsumeMetricsDefault(b *testing.B) {
	cfg := NewFactory().CreateDefaultConfig()
	benchmarkConsumeMetrics(b, cfg.(*Config))
}

func BenchmarkConsumeMetricsAll(b *testing.B) {
	cfg := &Config{Override: true, Detectors: []string{env.TypeStr, gce.TypeStr}}
	benchmarkConsumeMetrics(b, cfg)
}
