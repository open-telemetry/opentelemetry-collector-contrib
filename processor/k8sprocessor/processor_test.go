// Copyright 2020 OpenTelemetry Authors
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

package k8sprocessor

import (
	"context"
	"fmt"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

func TestNewTraceProcessor(t *testing.T) {
	_, err := NewTraceProcessor(
		zap.NewNop(),
		exportertest.NewNopTraceExporter(),
		newFakeClient,
	)
	require.NoError(t, err)
}

func TestTraceProcessorBadOption(t *testing.T) {
	opt := func(p *kubernetesprocessor) error {
		return fmt.Errorf("bad option")
	}
	p, err := NewTraceProcessor(
		zap.NewNop(),
		exportertest.NewNopTraceExporter(),
		newFakeClient,
		opt,
	)
	assert.Nil(t, p)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "bad option")
}

func TestTraceProcessorBadClientProvider(t *testing.T) {
	clientProvider := func(_ *zap.Logger, _ k8sconfig.APIConfig, _ kube.ExtractionRules, _ kube.Filters, _ kube.APIClientsetProvider, _ kube.InformerProvider) (kube.Client, error) {
		return nil, fmt.Errorf("bad client error")
	}
	p, err := NewTraceProcessor(
		zap.NewNop(),
		exportertest.NewNopTraceExporter(),
		clientProvider,
	)

	assert.Nil(t, p)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "bad client error")
}

func generateTraces() pdata.Traces {
	t := pdata.NewTraces()
	rs := t.ResourceSpans()
	rs.Resize(1)
	rs.At(0).InitEmpty()
	rs.At(0).InstrumentationLibrarySpans().Resize(1)
	rs.At(0).InstrumentationLibrarySpans().At(0).Spans().Resize(1)
	span := rs.At(0).InstrumentationLibrarySpans().At(0).Spans().At(0)
	span.SetName("foobar")
	return t
}

func TestIPDetection(t *testing.T) {
	next := &testTraceConsumer{}
	kp, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		newFakeClient,
	)
	require.NoError(t, err)

	ctx := client.NewContext(context.Background(), &client.Client{IP: "1.1.1.1"})
	err = kp.ConsumeTraces(ctx, generateTraces())
	require.NoError(t, err)

	require.Len(t, next.data, 1)
	rss := next.data[0].ResourceSpans()
	assert.Equal(t, 1, rss.Len())

	r := rss.At(0).Resource()
	require.False(t, r.IsNil())
	assertResourceHasStringAttribute(t, r, "k8s.pod.ip", "1.1.1.1")
}

func TestNilBatch(t *testing.T) {
	next := &testTraceConsumer{}
	kp, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		newFakeClient,
	)
	require.NoError(t, err)

	err = kp.ConsumeTraces(context.Background(), pdata.NewTraces())
	require.NoError(t, err)
	require.Len(t, next.data, 1)
}

func TestTraceProcessorNoAttrs(t *testing.T) {
	next := &testTraceConsumer{}
	p, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		newFakeClient,
		WithExtractMetadata(metadataPodName),
	)
	require.NoError(t, err)
	kp := p.(*kubernetesprocessor)
	kc := kp.kc.(*fakeClient)
	ctx := client.NewContext(context.Background(), &client.Client{IP: "1.1.1.1"})

	// pod doesn't have attrs to add
	kc.Pods["1.1.1.1"] = &kube.Pod{Name: "PodA"}
	p.ConsumeTraces(ctx, generateTraces())
	require.Len(t, next.data, 1)
	rss := next.data[0]
	rs := rss.ResourceSpans()
	assert.Equal(t, 1, rs.Len())
	assert.Equal(t, 1, rs.At(0).Resource().Attributes().Len())

	// attrs should be added now
	kc.Pods["1.1.1.1"] = &kube.Pod{
		Name: "PodA",
		Attributes: map[string]string{
			"k":  "v",
			"1":  "2",
			"aa": "b",
		},
	}
	next.data = []pdata.Traces{}
	p.ConsumeTraces(ctx, generateTraces())
	require.NoError(t, err)
	require.Len(t, next.data, 1)
	rss = next.data[0]
	rs = rss.ResourceSpans()
	assert.Equal(t, 1, rs.Len())
	assert.Equal(t, 4, rs.At(0).Resource().Attributes().Len())

	// passthrough doesn't add attrs
	next.data = []pdata.Traces{}
	kp.passthroughMode = true
	p.ConsumeTraces(ctx, generateTraces())
	require.Len(t, next.data, 1)
	rss = next.data[0]
	rs = rss.ResourceSpans()
	assert.Equal(t, 1, rs.Len())
	assert.Equal(t, 1, rs.At(0).Resource().Attributes().Len())

}

func TestNoIP(t *testing.T) {
	next := &testTraceConsumer{}
	kp, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		newFakeClient,
	)
	require.NoError(t, err)

	err = kp.ConsumeTraces(context.Background(), generateTraces())
	require.NoError(t, err)

	require.Len(t, next.data, 1)
	rss := next.data[0]
	rs := rss.ResourceSpans()
	assert.Equal(t, 1, rs.Len())
	assert.True(t, rs.At(0).Resource().IsNil())
}

func TestIPSource(t *testing.T) {
	next := &testTraceConsumer{}
	kp, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		newFakeClient,
	)
	require.NoError(t, err)

	type testCase struct {
		name, resourceIP, resourceK8SIP, contextIP, out string
	}

	testCases := []testCase{
		{
			name:          "k8sIP",
			resourceIP:    "1.1.1.1",
			resourceK8SIP: "2.2.2.2",
			contextIP:     "3.3.3.3",
			out:           "2.2.2.2",
		},
		{
			name:       "clientIP",
			resourceIP: "1.1.1.1",
			contextIP:  "3.3.3.3",
			out:        "1.1.1.1",
		},
		{
			name:      "contextIP",
			contextIP: "3.3.3.3",
			out:       "3.3.3.3",
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.contextIP != "" {
				ctx = client.NewContext(context.Background(), &client.Client{IP: tc.contextIP})
			}
			traces := generateTraces()
			resource := traces.ResourceSpans().At(0).Resource()
			if resource.IsNil() {
				resource.InitEmpty()
			}
			if tc.resourceK8SIP != "" {
				resource.Attributes().InsertString(k8sIPLabelName, tc.resourceK8SIP)
			}
			if tc.resourceIP != "" {
				resource.Attributes().InsertString(clientIPLabelName, tc.resourceIP)
			}
			err := kp.ConsumeTraces(ctx, traces)
			require.NoError(t, err)
			res := next.data[i].ResourceSpans().At(0).Resource()
			require.Len(t, next.data, i+1)
			require.False(t, res.IsNil())
			assertResourceHasStringAttribute(t, res, "k8s.pod.ip", tc.out)
		})
	}
}

func TestTraceProcessorAddLabels(t *testing.T) {
	next := &testTraceConsumer{}
	p, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		newFakeClient,
	)
	require.NoError(t, err)

	kp, ok := p.(*kubernetesprocessor)
	assert.True(t, ok)
	kc, ok := kp.kc.(*fakeClient)
	assert.True(t, ok)

	tests := map[string]map[string]string{
		"1": {
			"pod":         "test-2323",
			"ns":          "default",
			"another tag": "value",
		},
		"2": {},
	}
	for ip, attrs := range tests {
		kc.Pods[ip] = &kube.Pod{Attributes: attrs}
	}

	var i int
	for ip, attrs := range tests {
		ctx := client.NewContext(context.Background(), &client.Client{IP: ip})
		err = p.ConsumeTraces(ctx, generateTraces())
		require.NoError(t, err)

		require.Len(t, next.data, i+1)
		td := next.data[i]
		rss := td.ResourceSpans()
		require.Equal(t, rss.Len(), 1)
		r := rss.At(0).Resource()
		require.False(t, r.IsNil())
		assertResourceHasStringAttribute(t, r, "k8s.pod.ip", ip)
		for k, v := range attrs {
			assertResourceHasStringAttribute(t, r, k, v)
		}
		i++
	}
}

func TestPassthroughStart(t *testing.T) {
	next := &testTraceConsumer{}
	opts := []Option{WithPassthrough()}

	p, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		newFakeClient,
		opts...,
	)
	require.NoError(t, err)

	// Just make sure this doesn't fail when Passthrough is enabled
	assert.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, p.Shutdown(context.Background()))
}

func TestRealClient(t *testing.T) {
	p, err := NewTraceProcessor(
		zap.NewNop(),
		&testTraceConsumer{},
		nil,
		WithAPIConfig(k8sconfig.APIConfig{AuthType: "none"}),
	)
	assert.Nil(t, p)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "unable to load k8s config, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
}

func TestCapabilities(t *testing.T) {
	p, err := NewTraceProcessor(zap.NewNop(), &testTraceConsumer{}, newFakeClient)
	assert.NoError(t, err)
	caps := p.GetCapabilities()
	assert.True(t, caps.MutatesConsumedData)
}

func TestStartStop(t *testing.T) {
	next := &testTraceConsumer{}
	p, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		newFakeClient,
	)
	require.NoError(t, err)

	assert.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))

	pr := p.(*kubernetesprocessor)
	client := pr.kc.(*fakeClient)
	controller := client.Informer.GetController().(*kube.FakeController)

	assert.False(t, controller.HasStopped())
	assert.NoError(t, p.Shutdown(context.Background()))
	time.Sleep(time.Millisecond * 500)
	assert.True(t, controller.HasStopped())
}

func TestNewMetricsProcessor(t *testing.T) {
	_, err := NewMetricsProcessor(
		zap.NewNop(),
		exportertest.NewNopMetricsExporter(),
		newFakeClient,
	)
	require.NoError(t, err)
}

func TestMetricsProcessorBadOption(t *testing.T) {
	opt := func(p *kubernetesprocessor) error {
		return fmt.Errorf("bad option")
	}
	p, err := NewMetricsProcessor(
		zap.NewNop(),
		exportertest.NewNopMetricsExporter(),
		newFakeClient,
		opt,
	)
	assert.Nil(t, p)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "bad option")
}

func TestMetricsProcessorBadClientProvider(t *testing.T) {
	clientProvider := func(_ *zap.Logger, _ k8sconfig.APIConfig, _ kube.ExtractionRules, _ kube.Filters, _ kube.APIClientsetProvider, _ kube.InformerProvider) (kube.Client, error) {
		return nil, fmt.Errorf("bad client error")
	}
	p, err := NewMetricsProcessor(
		zap.NewNop(),
		exportertest.NewNopMetricsExporter(),
		clientProvider,
	)

	assert.Nil(t, p)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "bad client error")
}

func TestMetricsProcessorNoAttrs(t *testing.T) {
	next := &testMetricsConsumer{}
	p, err := NewMetricsProcessor(
		zap.NewNop(),
		next,
		newFakeClient,
		WithExtractMetadata(metadataPodName),
	)
	require.NoError(t, err)
	kp := p.(*kubernetesprocessor)
	kc := kp.kc.(*fakeClient)

	// pod doesn't have attrs to add
	kc.Pods["1.1.1.1"] = &kube.Pod{Name: "PodA"}
	metrics := generateMetrics()

	p.ConsumeMetrics(context.Background(), metrics)
	require.Len(t, next.data, 1)
	mds := pdatautil.MetricsToMetricsData(next.data[0])
	require.Equal(t, len(mds), 1)
	md := mds[0]
	require.Equal(t, 1, len(md.Resource.Labels))
	gotIP, ok := md.Resource.Labels["k8s.pod.ip"]
	assert.True(t, ok)
	assert.Equal(t, "1.1.1.1", gotIP)

	// attrs should be added now
	kc.Pods["1.1.1.1"] = &kube.Pod{
		Name: "PodA",
		Attributes: map[string]string{
			"k":  "v",
			"1":  "2",
			"aa": "b",
		},
	}

	p.ConsumeMetrics(context.Background(), metrics)
	require.Len(t, next.data, 2)
	mds = pdatautil.MetricsToMetricsData(next.data[1])
	require.Equal(t, len(mds), 1)
	md = mds[0]
	require.Equal(t, 4, len(md.Resource.Labels))
	gotIP, ok = md.Resource.Labels["k8s.pod.ip"]
	assert.True(t, ok)
	assert.Equal(t, "1.1.1.1", gotIP)
	gotAttr, ok := md.Resource.Labels["aa"]
	assert.True(t, ok)
	assert.Equal(t, "b", gotAttr)

	// passthrough doesn't add attrs
	metrics = generateMetrics()
	kp.passthroughMode = true
	p.ConsumeMetrics(context.Background(), metrics)
	require.Len(t, next.data, 3)
	mds = pdatautil.MetricsToMetricsData(next.data[2])
	require.Equal(t, len(mds), 1)
	md = mds[0]
	require.Equal(t, 1, len(md.Resource.Labels))
}

func TestMetricsProcessoInvalidIP(t *testing.T) {
	next := &testMetricsConsumer{}
	p, err := NewMetricsProcessor(
		zap.NewNop(),
		next,
		newFakeClient,
		WithExtractMetadata(metadataPodName),
	)
	require.NoError(t, err)
	kp := p.(*kubernetesprocessor)
	kc := kp.kc.(*fakeClient)

	// invalid ip should not be used to lookup k8s pod
	kc.Pods["invalid-ip"] = &kube.Pod{
		Name: "PodA",
		Attributes: map[string]string{
			"k":  "v",
			"1":  "2",
			"aa": "b",
		},
	}
	metrics := generateMetrics()
	md := pdatautil.MetricsToMetricsData(metrics)[0]
	md.Node.Identifier.HostName = "invalid-ip"

	p.ConsumeMetrics(context.Background(), metrics)
	require.Len(t, next.data, 1)
	mds := pdatautil.MetricsToMetricsData(next.data[0])
	require.Equal(t, len(mds), 1)
	md = mds[0]
	require.Nil(t, md.Resource)
}

func TestMetricsProcessorAddLabels(t *testing.T) {
	next := &testMetricsConsumer{}
	p, err := NewMetricsProcessor(
		zap.NewNop(),
		next,
		newFakeClient,
	)
	require.NoError(t, err)

	kp, ok := p.(*kubernetesprocessor)
	assert.True(t, ok)
	kc, ok := kp.kc.(*fakeClient)
	assert.True(t, ok)

	tests := map[string]map[string]string{
		"1.2.3.4": {
			"pod":         "test-2323",
			"ns":          "default",
			"another tag": "value",
		},
		"2.3.4.5": {
			"pod": "test-12",
		},
	}
	for ip, attrs := range tests {
		kc.Pods[ip] = &kube.Pod{Attributes: attrs}
	}

	var i int
	for ip, attrs := range tests {
		metrics := generateMetrics()
		md := pdatautil.MetricsToMetricsData(metrics)[0]
		md.Node.Identifier.HostName = ip

		err = p.ConsumeMetrics(context.Background(), metrics)
		require.NoError(t, err)

		require.Len(t, next.data, i+1)
		mds := pdatautil.MetricsToMetricsData(next.data[i])
		require.Equal(t, len(mds), 1)
		md = mds[0]
		require.Equal(t, len(attrs)+1, len(md.Resource.Labels))
		gotIP, ok := md.Resource.Labels["k8s.pod.ip"]
		assert.True(t, ok)
		assert.Equal(t, ip, gotIP)
		for k, v := range attrs {
			got, ok := attrs[k]
			assert.True(t, ok)
			assert.Equal(t, v, got)
		}
		i++
	}
}

type testTraceConsumer struct {
	data []pdata.Traces
}

func (tc *testTraceConsumer) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	tc.data = append(tc.data, td)
	return nil
}

type testMetricsConsumer struct {
	data []pdata.Metrics
}

func (mc *testMetricsConsumer) ConsumeMetrics(ctx context.Context, td pdata.Metrics) error {
	mc.data = append(mc.data, td)
	return nil
}

func generateMetrics() pdata.Metrics {
	md := consumerdata.MetricsData{
		Node: &commonpb.Node{
			Identifier: &commonpb.ProcessIdentifier{
				HostName: "1.1.1.1",
			},
		},
		Metrics: []*metricspb.Metric{
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: "my-metric",
					Type: metricspb.MetricDescriptor_GAUGE_INT64,
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						Points: []*metricspb.Point{
							{Value: &metricspb.Point_Int64Value{Int64Value: 123}},
						},
					},
				},
			},
		},
	}
	return pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{md})
}

func assertResourceHasStringAttribute(t *testing.T, r pdata.Resource, k, v string) {
	got, ok := r.Attributes().Get(k)
	assert.True(t, ok, fmt.Sprintf("resource does not contain attribute %s", k))
	assert.EqualValues(t, pdata.AttributeValueSTRING, got.Type(), "attribute %s is not of type string", k)
	assert.EqualValues(t, v, got.StringVal(), "attribute %s is not equal to %s", k, v)
}
