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
	"errors"
	"testing"
	"time"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"

	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/env"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/gce"
)

type MockDetector struct {
	mock.Mock
}

func (p *MockDetector) Detect(ctx context.Context) (resource pdata.Resource, schemaURL string, err error) {
	args := p.Called()
	return args.Get(0).(pdata.Resource), "", args.Error(1)
}

func TestResourceProcessor(t *testing.T) {
	tests := []struct {
		name               string
		detectorKeys       []string
		override           bool
		sourceResource     pdata.Resource
		detectedResource   pdata.Resource
		detectedError      error
		expectedResource   pdata.Resource
		expectedNewError   string
		expectedStartError string
	}{
		{
			name:     "Resource is not overridden",
			override: false,
			sourceResource: internal.NewResource(map[string]interface{}{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "original-zone",
			}),
			detectedResource: internal.NewResource(map[string]interface{}{
				"cloud.availability_zone": "will-be-ignored",
				"k8s.cluster.name":        "k8s-cluster",
				"host.name":               "k8s-node",
				"bool":                    true,
				"int":                     int64(100),
				"double":                  0.1,
			}),
			expectedResource: internal.NewResource(map[string]interface{}{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "original-zone",
				"k8s.cluster.name":        "k8s-cluster",
				"host.name":               "k8s-node",
				"bool":                    true,
				"int":                     int64(100),
				"double":                  0.1,
			}),
		},
		{
			name:     "Resource is overridden",
			override: true,
			sourceResource: internal.NewResource(map[string]interface{}{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "will-be-overridden",
			}),
			detectedResource: internal.NewResource(map[string]interface{}{
				"cloud.availability_zone": "zone-1",
				"k8s.cluster.name":        "k8s-cluster",
				"host.name":               "k8s-node",
			}),
			expectedResource: internal.NewResource(map[string]interface{}{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "zone-1",
				"k8s.cluster.name":        "k8s-cluster",
				"host.name":               "k8s-node",
			}),
		},
		{
			name: "Empty detected resource",
			sourceResource: internal.NewResource(map[string]interface{}{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "original-zone",
			}),
			detectedResource: internal.NewResource(map[string]interface{}{}),
			expectedResource: internal.NewResource(map[string]interface{}{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "original-zone",
			}),
		},
		{
			name:             "Source resource is nil",
			sourceResource:   pdata.NewResource(),
			detectedResource: internal.NewResource(map[string]interface{}{"host.name": "node"}),
			expectedResource: internal.NewResource(map[string]interface{}{"host.name": "node"}),
		},
		{
			name:             "Detected resource is nil",
			sourceResource:   internal.NewResource(map[string]interface{}{"host.name": "node"}),
			detectedResource: pdata.NewResource(),
			expectedResource: internal.NewResource(map[string]interface{}{"host.name": "node"}),
		},
		{
			name:             "Both resources are nil",
			sourceResource:   pdata.NewResource(),
			detectedResource: pdata.NewResource(),
			expectedResource: internal.NewResource(map[string]interface{}{}),
		},
		{
			name: "Detection error",
			sourceResource: internal.NewResource(map[string]interface{}{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "original-zone",
			}),
			detectedError:      errors.New("err1"),
			expectedStartError: "err1",
		},
		{
			name:             "Invalid detector key",
			detectorKeys:     []string{"invalid-key"},
			expectedNewError: "invalid detector key: invalid-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := &factory{providers: map[config.ComponentID]*internal.ResourceProvider{}}

			md1 := &MockDetector{}
			md1.On("Detect").Return(tt.detectedResource, tt.detectedError)
			factory.resourceProviderFactory = internal.NewProviderFactory(
				map[internal.DetectorType]internal.DetectorFactory{"mock": func(component.ProcessorCreateSettings, internal.DetectorConfig) (internal.Detector, error) {
					return md1, nil
				}})

			if tt.detectorKeys == nil {
				tt.detectorKeys = []string{"mock"}
			}

			cfg := &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewID(typeStr)),
				Override:          tt.override,
				Detectors:         tt.detectorKeys,
				Timeout:           time.Second,
			}

			// Test trace consuner
			ttn := new(consumertest.TracesSink)
			rtp, err := factory.createTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, ttn)

			if tt.expectedNewError != "" {
				assert.EqualError(t, err, tt.expectedNewError)
				return
			}

			require.NoError(t, err)
			assert.True(t, rtp.Capabilities().MutatesData)

			err = rtp.Start(context.Background(), componenttest.NewNopHost())

			if tt.expectedStartError != "" {
				assert.EqualError(t, err, tt.expectedStartError)
				return
			}

			require.NoError(t, err)
			defer func() { assert.NoError(t, rtp.Shutdown(context.Background())) }()

			td := pdata.NewTraces()
			tt.sourceResource.CopyTo(td.ResourceSpans().AppendEmpty().Resource())

			err = rtp.ConsumeTraces(context.Background(), td)
			require.NoError(t, err)
			got := ttn.AllTraces()[0].ResourceSpans().At(0).Resource()

			tt.expectedResource.Attributes().Sort()
			got.Attributes().Sort()
			assert.Equal(t, tt.expectedResource, got)

			// Test metrics consumer
			tmn := new(consumertest.MetricsSink)
			rmp, err := factory.createMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, tmn)

			if tt.expectedNewError != "" {
				assert.EqualError(t, err, tt.expectedNewError)
				return
			}

			require.NoError(t, err)
			assert.True(t, rmp.Capabilities().MutatesData)

			err = rmp.Start(context.Background(), componenttest.NewNopHost())

			if tt.expectedStartError != "" {
				assert.EqualError(t, err, tt.expectedStartError)
				return
			}

			require.NoError(t, err)
			defer func() { assert.NoError(t, rmp.Shutdown(context.Background())) }()

			// TODO create pdata.Metrics directly when this is no longer internal
			err = rmp.ConsumeMetrics(context.Background(), internaldata.OCToMetrics(nil, oCensusResource(tt.sourceResource), nil))
			require.NoError(t, err)
			got = tmn.AllMetrics()[0].ResourceMetrics().At(0).Resource()

			tt.expectedResource.Attributes().Sort()
			got.Attributes().Sort()
			assert.Equal(t, tt.expectedResource, got)

			// Test logs consumer
			tln := new(consumertest.LogsSink)
			rlp, err := factory.createLogsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, tln)

			if tt.expectedNewError != "" {
				assert.EqualError(t, err, tt.expectedNewError)
				return
			}

			require.NoError(t, err)
			assert.True(t, rlp.Capabilities().MutatesData)

			err = rlp.Start(context.Background(), componenttest.NewNopHost())

			if tt.expectedStartError != "" {
				assert.EqualError(t, err, tt.expectedStartError)
				return
			}

			require.NoError(t, err)
			defer func() { assert.NoError(t, rlp.Shutdown(context.Background())) }()

			ld := pdata.NewLogs()
			tt.sourceResource.CopyTo(ld.ResourceLogs().AppendEmpty().Resource())

			err = rlp.ConsumeLogs(context.Background(), ld)
			require.NoError(t, err)
			got = tln.AllLogs()[0].ResourceLogs().At(0).Resource()

			tt.expectedResource.Attributes().Sort()
			got.Attributes().Sort()
			assert.Equal(t, tt.expectedResource, got)
		})
	}
}

func oCensusResource(res pdata.Resource) *resourcepb.Resource {
	if res.Attributes().Len() == 0 {
		return &resourcepb.Resource{}
	}

	mp := make(map[string]string, res.Attributes().Len())
	res.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
		mp[k] = v.StringVal()
		return true
	})

	return &resourcepb.Resource{Labels: mp}
}

func benchmarkConsumeTraces(b *testing.B, cfg *Config) {
	factory := NewFactory()
	sink := new(consumertest.TracesSink)
	processor, _ := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, sink)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// TODO use testbed.PerfTestDataProvider here once that includes resources
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
	factory := NewFactory()
	sink := new(consumertest.MetricsSink)
	processor, _ := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, sink)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// TODO use testbed.PerfTestDataProvider here once that includes resources
		processor.ConsumeMetrics(context.Background(), pdata.NewMetrics())
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

func benchmarkConsumeLogs(b *testing.B, cfg *Config) {
	factory := NewFactory()
	sink := new(consumertest.LogsSink)
	processor, _ := factory.CreateLogsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, sink)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// TODO use testbed.PerfTestDataProvider here once that includes resources
		processor.ConsumeLogs(context.Background(), pdata.NewLogs())
	}
}

func BenchmarkConsumeLogsDefault(b *testing.B) {
	cfg := NewFactory().CreateDefaultConfig()
	benchmarkConsumeLogs(b, cfg.(*Config))
}

func BenchmarkConsumeLogsAll(b *testing.B) {
	cfg := &Config{Override: true, Detectors: []string{env.TypeStr, gce.TypeStr}}
	benchmarkConsumeLogs(b, cfg)
}
