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
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

func newTraceProcessor(cfg configmodels.Processor, nextTraceConsumer consumer.TraceConsumer, options ...Option) (component.TraceProcessor, error) {
	opts := append(options, withKubeClientProvider(newFakeClient))
	return createTraceProcessorWithOptions(
		context.Background(),
		component.ProcessorCreateParams{Logger: zap.NewNop()},
		cfg,
		nextTraceConsumer,
		opts...,
	)
}

func newMetricsProcessor(cfg configmodels.Processor, nextMetricsConsumer consumer.MetricsConsumer, options ...Option) (component.MetricsProcessor, error) {
	opts := append(options, withKubeClientProvider(newFakeClient))
	return createMetricsProcessorWithOptions(
		context.Background(),
		component.ProcessorCreateParams{Logger: zap.NewNop()},
		cfg,
		nextMetricsConsumer,
		opts...,
	)
}

func newLogsProcessor(cfg configmodels.Processor, nextLogsConsumer consumer.LogsConsumer, options ...Option) (component.LogsProcessor, error) {
	opts := append(options, withKubeClientProvider(newFakeClient))
	return createLogsProcessorWithOptions(
		context.Background(),
		component.ProcessorCreateParams{Logger: zap.NewNop()},
		cfg,
		nextLogsConsumer,
		opts...,
	)
}

// withKubeClientProvider sets the specific implementation for getting K8s Client instances
func withKubeClientProvider(kcp kube.ClientProvider) Option {
	return func(p *kubernetesprocessor) error {
		return p.initKubeClient(p.logger, kcp)
	}
}

// withExtractKubernetesProcessorInto allows to pull the internal model easily even when processorhelper factory is used
func withExtractKubernetesProcessorInto(kp **kubernetesprocessor) Option {
	return func(p *kubernetesprocessor) error {
		*kp = p
		return nil
	}
}

type multiTest struct {
	t *testing.T

	tp component.TraceProcessor
	mp component.MetricsProcessor
	lp component.LogsProcessor

	nextTrace   *exportertest.SinkTraceExporter
	nextMetrics *exportertest.SinkMetricsExporter
	nextLogs    *exportertest.SinkLogsExporter

	kpTrace   *kubernetesprocessor
	kpMetrics *kubernetesprocessor
	kpLogs    *kubernetesprocessor
}

func newMultiTest(
	t *testing.T,
	cfg configmodels.Processor,
	errFunc func(err error),
	options ...Option,
) *multiTest {
	m := &multiTest{
		t:           t,
		nextTrace:   &exportertest.SinkTraceExporter{},
		nextMetrics: &exportertest.SinkMetricsExporter{},
		nextLogs:    &exportertest.SinkLogsExporter{},
	}

	tp, err := newTraceProcessor(cfg, m.nextTrace, append(options, withExtractKubernetesProcessorInto(&m.kpTrace))...)
	if errFunc == nil {
		assert.NotNil(t, tp)
		require.NoError(t, err)
	} else {
		assert.Nil(t, tp)
		errFunc(err)
	}

	mp, err := newMetricsProcessor(cfg, m.nextMetrics, append(options, withExtractKubernetesProcessorInto(&m.kpMetrics))...)
	if errFunc == nil {
		assert.NotNil(t, mp)
		require.NoError(t, err)
	} else {
		assert.Nil(t, mp)
		errFunc(err)
	}

	lp, err := newLogsProcessor(cfg, m.nextLogs, append(options, withExtractKubernetesProcessorInto(&m.kpLogs))...)
	if errFunc == nil {
		assert.NotNil(t, lp)
		require.NoError(t, err)
	} else {
		assert.Nil(t, lp)
		errFunc(err)
	}

	m.tp = tp
	m.mp = mp
	m.lp = lp
	return m
}

func (m *multiTest) testConsume(
	ctx context.Context,
	traces pdata.Traces,
	metrics pdata.Metrics,
	logs pdata.Logs,
	errFunc func(err error),
) {
	errs := []error{
		m.tp.ConsumeTraces(ctx, traces),
		m.mp.ConsumeMetrics(ctx, metrics),
		m.lp.ConsumeLogs(ctx, logs),
	}

	for _, err := range errs {
		if errFunc != nil {
			errFunc(err)
		}
	}
}

func (m *multiTest) kubernetesProcessorOperation(kpOp func(kp *kubernetesprocessor)) {
	kpOp(m.kpTrace)
	kpOp(m.kpMetrics)
	kpOp(m.kpLogs)
}

func (m *multiTest) assertBatchesLen(batchesLen int) {
	require.Len(m.t, m.nextTrace.AllTraces(), batchesLen)
	require.Len(m.t, m.nextMetrics.AllMetrics(), batchesLen)
	require.Len(m.t, m.nextLogs.AllLogs(), batchesLen)
}

func (m *multiTest) assertResourceObjectLen(batchNo int, rsObjectLen int) {
	assert.Equal(m.t, m.nextTrace.AllTraces()[batchNo].ResourceSpans().Len(), rsObjectLen)
	assert.Equal(m.t, m.nextMetrics.AllMetrics()[batchNo].ResourceMetrics().Len(), rsObjectLen)
	assert.Equal(m.t, m.nextLogs.AllLogs()[batchNo].ResourceLogs().Len(), rsObjectLen)
}

func (m *multiTest) assertResourceAttributesLen(batchNo int, resourceObjectNo int, attrsLen int) {
	assert.Equal(m.t, m.nextTrace.AllTraces()[batchNo].ResourceSpans().At(resourceObjectNo).Resource().Attributes().Len(), attrsLen)
	assert.Equal(m.t, m.nextMetrics.AllMetrics()[batchNo].ResourceMetrics().At(resourceObjectNo).Resource().Attributes().Len(), attrsLen)
	assert.Equal(m.t, m.nextLogs.AllLogs()[batchNo].ResourceLogs().At(resourceObjectNo).Resource().Attributes().Len(), attrsLen)
}

func (m *multiTest) assertResource(batchNum int, resourceXNum int, resourceFunc func(res pdata.Resource)) {
	rss := m.nextTrace.AllTraces()[batchNum].ResourceSpans()
	r := rss.At(resourceXNum).Resource()

	if resourceFunc != nil {
		resourceFunc(r)
	}
}

func TestNewProcessor(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()

	newMultiTest(t, cfg, nil)
}

func TestProcessorBadConfig(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Extract.Metadata = []string{"bad-attribute"}

	newMultiTest(t, cfg, func(err error) {
		assert.Error(t, err)
		assert.Equal(t, err.Error(), "\"bad-attribute\" is not a supported metadata field")
	})
}

func TestProcessorBadClientProvider(t *testing.T) {
	clientProvider := func(_ *zap.Logger, _ k8sconfig.APIConfig, _ kube.ExtractionRules, _ kube.Filters, _ kube.APIClientsetProvider, _ kube.InformerProvider) (kube.Client, error) {
		return nil, fmt.Errorf("bad client error")
	}

	newMultiTest(t, NewFactory().CreateDefaultConfig(), func(err error) {
		assert.Error(t, err)
		assert.Equal(t, err.Error(), "bad client error")
	}, withKubeClientProvider(clientProvider))
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

func generateMetrics() pdata.Metrics {
	m := pdata.NewMetrics()
	ms := m.ResourceMetrics()
	ms.Resize(1)
	ms.At(0).InitEmpty()
	ms.At(0).InstrumentationLibraryMetrics().Resize(1)
	ms.At(0).InstrumentationLibraryMetrics().At(0).Metrics().Resize(1)
	metric := ms.At(0).InstrumentationLibraryMetrics().At(0).Metrics().At(0)
	metric.SetName("foobar")
	return m
}

func generateLogs() pdata.Logs {
	l := pdata.NewLogs()
	ls := l.ResourceLogs()
	ls.Resize(1)
	ls.At(0).InitEmpty()
	ls.At(0).InstrumentationLibraryLogs().Resize(1)
	ls.At(0).InstrumentationLibraryLogs().At(0).Logs().Resize(1)
	log := ls.At(0).InstrumentationLibraryLogs().At(0).Logs().At(0)
	log.SetName("foobar")
	return l
}

func TestIPDetection(t *testing.T) {
	m := newMultiTest(t, NewFactory().CreateDefaultConfig(), nil)

	ctx := client.NewContext(context.Background(), &client.Client{IP: "1.1.1.1"})
	m.testConsume(
		ctx,
		generateTraces(),
		generateMetrics(),
		generateLogs(),
		func(err error) {
			assert.NoError(t, err)
		})

	m.assertBatchesLen(1)
	m.assertResourceObjectLen(0, 1)
	m.assertResource(0, 0, func(r pdata.Resource) {
		require.False(t, r.IsNil())
		assertResourceHasStringAttribute(t, r, "k8s.pod.ip", "1.1.1.1")
	})
}

func TestNilBatch(t *testing.T) {
	m := newMultiTest(t, NewFactory().CreateDefaultConfig(), nil)
	m.testConsume(
		context.Background(),
		pdata.NewTraces(),
		pdata.NewMetrics(),
		pdata.NewLogs(),
		func(err error) {
			assert.NoError(t, err)
		})

	m.assertBatchesLen(1)
}

func TestTraceProcessorNoAttrs(t *testing.T) {
	m := newMultiTest(
		t,
		NewFactory().CreateDefaultConfig(),
		nil,
		WithExtractMetadata(metadataPodName),
	)

	ctx := client.NewContext(context.Background(), &client.Client{IP: "1.1.1.1"})

	// pod doesn't have attrs to add
	m.kubernetesProcessorOperation(func(kp *kubernetesprocessor) {
		kp.kc.(*fakeClient).Pods["1.1.1.1"] = &kube.Pod{Name: "PodA"}
	})

	m.testConsume(
		ctx,
		generateTraces(),
		generateMetrics(),
		generateLogs(),
		func(err error) {
			assert.NoError(t, err)
		})

	m.assertBatchesLen(1)
	m.assertResourceObjectLen(0, 1)
	m.assertResourceAttributesLen(0, 0, 1)

	// attrs should be added now
	m.kubernetesProcessorOperation(func(kp *kubernetesprocessor) {
		kp.kc.(*fakeClient).Pods["1.1.1.1"] = &kube.Pod{
			Name: "PodA",
			Attributes: map[string]string{
				"k":  "v",
				"1":  "2",
				"aa": "b",
			},
		}
	})

	m.testConsume(
		ctx,
		generateTraces(),
		generateMetrics(),
		generateLogs(),
		func(err error) {
			assert.NoError(t, err)
		})

	m.assertBatchesLen(2)
	m.assertResourceObjectLen(1, 1)
	m.assertResourceAttributesLen(1, 0, 4)

	// passthrough doesn't add attrs
	m.kubernetesProcessorOperation(func(kp *kubernetesprocessor) {
		kp.passthroughMode = true
	})
	m.testConsume(
		ctx,
		generateTraces(),
		generateMetrics(),
		generateLogs(),
		func(err error) {
			assert.NoError(t, err)
		})

	m.assertBatchesLen(3)
	m.assertResourceObjectLen(2, 1)
	m.assertResourceAttributesLen(2, 0, 1)
}

func TestNoIP(t *testing.T) {
	m := newMultiTest(
		t,
		NewFactory().CreateDefaultConfig(),
		nil,
	)

	m.testConsume(context.Background(), generateTraces(), generateMetrics(), generateLogs(), nil)

	m.assertBatchesLen(1)
	m.assertResourceObjectLen(0, 1)
	m.assertResource(0, 0, func(res pdata.Resource) {
		assert.True(t, res.IsNil())
	})
}

func TestIPSource(t *testing.T) {
	m := newMultiTest(
		t,
		NewFactory().CreateDefaultConfig(),
		nil,
	)

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
			metrics := generateMetrics()
			logs := generateLogs()

			resources := []pdata.Resource{
				traces.ResourceSpans().At(0).Resource(),
				metrics.ResourceMetrics().At(0).Resource(),
				logs.ResourceLogs().At(0).Resource(),
			}

			for _, res := range resources {
				if res.IsNil() {
					res.InitEmpty()
				}
				if tc.resourceK8SIP != "" {
					res.Attributes().InsertString(k8sIPLabelName, tc.resourceK8SIP)
				}
				if tc.resourceIP != "" {
					res.Attributes().InsertString(clientIPLabelName, tc.resourceIP)
				}
			}

			m.testConsume(ctx, traces, metrics, logs, nil)
			m.assertBatchesLen(i + 1)
			m.assertResource(i, 0, func(res pdata.Resource) {
				require.False(t, res.IsNil())
				assertResourceHasStringAttribute(t, res, "k8s.pod.ip", tc.out)
			})
		})
	}
}

func TestTraceProcessorAddLabels(t *testing.T) {
	next := &exportertest.SinkTraceExporter{}
	var kp *kubernetesprocessor
	p, err := newTraceProcessor(
		NewFactory().CreateDefaultConfig(),
		next,
		withExtractKubernetesProcessorInto(&kp),
	)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.NotNil(t, kp)
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

		require.Len(t, next.AllTraces(), i+1)
		td := next.AllTraces()[i]
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
	next := &exportertest.SinkTraceExporter{}
	opts := []Option{WithPassthrough()}

	p, err := newTraceProcessor(
		NewFactory().CreateDefaultConfig(),
		next,
		opts...,
	)
	require.NoError(t, err)

	// Just make sure this doesn't fail when Passthrough is enabled
	assert.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, p.Shutdown(context.Background()))
}

func TestRealClient(t *testing.T) {
	newMultiTest(
		t,
		NewFactory().CreateDefaultConfig(),
		func(err error) {
			assert.Error(t, err)
			assert.Equal(t, err.Error(), "unable to load k8s config, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
		},
		withKubeClientProvider(kubeClientProvider),
		WithAPIConfig(k8sconfig.APIConfig{AuthType: "none"}),
	)
}

func TestCapabilities(t *testing.T) {
	p, err := newTraceProcessor(
		NewFactory().CreateDefaultConfig(),
		exportertest.NewNopTraceExporter(),
	)
	assert.NoError(t, err)
	caps := p.GetCapabilities()
	assert.True(t, caps.MutatesConsumedData)
}

func TestStartStop(t *testing.T) {
	var kp *kubernetesprocessor
	p, err := newTraceProcessor(
		NewFactory().CreateDefaultConfig(),
		exportertest.NewNopTraceExporter(),
		withExtractKubernetesProcessorInto(&kp),
	)
	require.NoError(t, err)

	assert.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))

	assert.NotNil(t, kp)
	kc := kp.kc.(*fakeClient)
	controller := kc.Informer.GetController().(*kube.FakeController)

	assert.False(t, controller.HasStopped())
	assert.NoError(t, p.Shutdown(context.Background()))
	time.Sleep(time.Millisecond * 500)
	assert.True(t, controller.HasStopped())
}

func TestMetricsProcessorNoAttrs(t *testing.T) {
	next := &exportertest.SinkMetricsExporter{}
	var kp *kubernetesprocessor
	p, err := newMetricsProcessor(
		NewFactory().CreateDefaultConfig(),
		next,
		WithExtractMetadata(metadataPodName),
		withExtractKubernetesProcessorInto(&kp),
	)
	require.NoError(t, err)
	kc := kp.kc.(*fakeClient)

	// pod doesn't have attrs to add
	kc.Pods["1.1.1.1"] = &kube.Pod{Name: "PodA"}
	metrics := generateMetricsWithHostname()

	assert.NoError(t, p.ConsumeMetrics(context.Background(), metrics))
	require.Len(t, next.AllMetrics(), 1)
	mds := internaldata.MetricsToOC(next.AllMetrics()[0])
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

	assert.NoError(t, p.ConsumeMetrics(context.Background(), metrics))
	require.Len(t, next.AllMetrics(), 2)
	mds = internaldata.MetricsToOC(next.AllMetrics()[1])
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
	metrics = generateMetricsWithHostname()
	kp.passthroughMode = true
	assert.NoError(t, p.ConsumeMetrics(context.Background(), metrics))
	require.Len(t, next.AllMetrics(), 3)
	mds = internaldata.MetricsToOC(next.AllMetrics()[2])
	require.Equal(t, len(mds), 1)
	md = mds[0]
	require.Equal(t, 1, len(md.Resource.Labels))
}

func TestMetricsProcessorPicksUpPassthoughPodIp(t *testing.T) {
	next := &exportertest.SinkMetricsExporter{}
	var kp *kubernetesprocessor
	p, err := newMetricsProcessor(
		NewFactory().CreateDefaultConfig(),
		next,
		WithExtractMetadata(metadataPodName),
		withExtractKubernetesProcessorInto(&kp),
	)
	require.NoError(t, err)
	kc := kp.kc.(*fakeClient)
	kc.Pods["2.2.2.2"] = &kube.Pod{
		Name: "PodA",
		Attributes: map[string]string{
			"k": "v",
			"1": "2",
		},
	}

	metrics := generateMetricsWithPodIP()

	assert.NoError(t, p.ConsumeMetrics(context.Background(), metrics))
	require.Len(t, next.AllMetrics(), 1)
	mds := internaldata.MetricsToOC(next.AllMetrics()[0])
	require.Equal(t, len(mds), 1)
	md := mds[0]
	require.Equal(t, 3, len(md.Resource.Labels))
	require.Equal(t, "2.2.2.2", md.Resource.Labels[k8sIPLabelName])
	require.Equal(t, "v", md.Resource.Labels["k"])
	require.Equal(t, "2", md.Resource.Labels["1"])
}

func TestMetricsProcessorInvalidIP(t *testing.T) {
	next := &exportertest.SinkMetricsExporter{}
	var kp *kubernetesprocessor
	p, err := newMetricsProcessor(
		NewFactory().CreateDefaultConfig(),
		next,
		WithExtractMetadata(metadataPodName),
		withExtractKubernetesProcessorInto(&kp),
	)
	require.NoError(t, err)
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
	metrics := generateMetricsWithHostname()
	mds := internaldata.MetricsToOC(metrics)
	require.Len(t, mds, 1)
	mds[0].Node.Identifier.HostName = "invalid-ip"

	assert.NoError(t, p.ConsumeMetrics(context.Background(), internaldata.OCSliceToMetrics(mds)))
	require.Len(t, next.AllMetrics(), 1)
	mds = internaldata.MetricsToOC(next.AllMetrics()[0])
	require.Equal(t, len(mds), 1)
	md := mds[0]
	assert.Len(t, md.Resource.Labels, 0)
}

func TestMetricsProcessorAddLabels(t *testing.T) {
	next := &exportertest.SinkMetricsExporter{}
	var kp *kubernetesprocessor
	p, err := newMetricsProcessor(
		NewFactory().CreateDefaultConfig(),
		next,
		withExtractKubernetesProcessorInto(&kp),
	)
	require.NoError(t, err)

	assert.NotNil(t, kp)
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

	for ip := range tests {
		attrs := tests[ip]
		t.Run(ip, func(t *testing.T) {
			next.Reset()
			metrics := generateMetricsWithHostname()
			mds := internaldata.MetricsToOC(metrics)
			mds[0].Node.Identifier.HostName = ip

			err = p.ConsumeMetrics(context.Background(), internaldata.OCSliceToMetrics(mds))
			require.NoError(t, err)

			require.Len(t, next.AllMetrics(), 1)
			mds = internaldata.MetricsToOC(next.AllMetrics()[0])
			require.Len(t, mds, 1)
			md := mds[0]
			require.Lenf(t, md.Resource.Labels, len(attrs)+1, "%v", md.Node)
			gotIP, ok := md.Resource.Labels["k8s.pod.ip"]
			assert.True(t, ok)
			assert.Equal(t, ip, gotIP)
			for k, v := range attrs {
				got, ok := attrs[k]
				assert.True(t, ok)
				assert.Equal(t, v, got)
			}
		})
	}
}

func generateMetricsWithHostname() pdata.Metrics {
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
	return internaldata.OCToMetrics(md)
}

func generateMetricsWithPodIP() pdata.Metrics {
	md := consumerdata.MetricsData{
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				k8sIPLabelName: "2.2.2.2",
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
	return internaldata.OCToMetrics(md)
}

func assertResourceHasStringAttribute(t *testing.T, r pdata.Resource, k, v string) {
	got, ok := r.Attributes().Get(k)
	assert.True(t, ok, fmt.Sprintf("resource does not contain attribute %s", k))
	assert.EqualValues(t, pdata.AttributeValueSTRING, got.Type(), "attribute %s is not of type string", k)
	assert.EqualValues(t, v, got.StringVal(), "attribute %s is not equal to %s", k, v)
}
