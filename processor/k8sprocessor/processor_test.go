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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

func newTraceProcessor(cfg configmodels.Processor, next consumer.TracesConsumer, options ...Option) (component.TraceProcessor, error) {
	opts := append(options, withKubeClientProvider(newFakeClient))
	return createTraceProcessorWithOptions(
		context.Background(),
		component.ProcessorCreateParams{Logger: zap.NewNop()},
		cfg,
		next,
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

	nextTrace   *consumertest.TracesSink
	nextMetrics *consumertest.MetricsSink
	nextLogs    *consumertest.LogsSink

	kpMetrics *kubernetesprocessor
	kpTrace   *kubernetesprocessor
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
		nextTrace:   new(consumertest.TracesSink),
		nextMetrics: new(consumertest.MetricsSink),
		nextLogs:    new(consumertest.LogsSink),
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

func (m *multiTest) assertResource(batchNum int, resourceObjectNum int, resourceFunc func(res pdata.Resource)) {
	rss := m.nextTrace.AllTraces()[batchNum].ResourceSpans()
	r := rss.At(resourceObjectNum).Resource()

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

type generateResourceFunc func(res pdata.Resource)

func generateTraces(resourceFunc ...generateResourceFunc) pdata.Traces {
	t := pdata.NewTraces()
	rs := t.ResourceSpans()
	rs.Resize(1)
	rs.At(0).InitEmpty()
	rs.At(0).InstrumentationLibrarySpans().Resize(1)
	rs.At(0).InstrumentationLibrarySpans().At(0).Spans().Resize(1)
	for _, resFun := range resourceFunc {
		res := rs.At(0).Resource()
		if res.IsNil() {
			res.InitEmpty()
		}
		resFun(res)
	}
	span := rs.At(0).InstrumentationLibrarySpans().At(0).Spans().At(0)
	span.SetName("foobar")
	return t
}

func generateMetrics(resourceFunc ...generateResourceFunc) pdata.Metrics {
	m := pdata.NewMetrics()
	ms := m.ResourceMetrics()
	ms.Resize(1)
	ms.At(0).InitEmpty()
	ms.At(0).InstrumentationLibraryMetrics().Resize(1)
	ms.At(0).InstrumentationLibraryMetrics().At(0).Metrics().Resize(1)
	for _, resFun := range resourceFunc {
		res := ms.At(0).Resource()
		if res.IsNil() {
			res.InitEmpty()
		}
		resFun(res)
	}
	metric := ms.At(0).InstrumentationLibraryMetrics().At(0).Metrics().At(0)
	metric.SetName("foobar")
	return m
}

func generateLogs(resourceFunc ...generateResourceFunc) pdata.Logs {
	l := pdata.NewLogs()
	ls := l.ResourceLogs()
	ls.Resize(1)
	ls.At(0).InitEmpty()
	ls.At(0).InstrumentationLibraryLogs().Resize(1)
	ls.At(0).InstrumentationLibraryLogs().At(0).Logs().Resize(1)
	for _, resFun := range resourceFunc {
		res := ls.At(0).Resource()
		if res.IsNil() {
			res.InitEmpty()
		}
		resFun(res)
	}
	log := ls.At(0).InstrumentationLibraryLogs().At(0).Logs().At(0)
	log.SetName("foobar")
	return l
}

func withPassthroughIP(passthroughIP string) generateResourceFunc {
	return func(res pdata.Resource) {
		res.Attributes().InsertString(k8sIPLabelName, passthroughIP)
	}
}

func withHostname(hostname string) generateResourceFunc {
	return func(res pdata.Resource) {
		res.Attributes().InsertString(conventions.AttributeHostHostname, hostname)
	}
}

func TestIPDetectionFromContext(t *testing.T) {
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
		generateLogs(),
		func(err error) {
			assert.NoError(t, err)
		})

	m.assertBatchesLen(1)
}

func TestProcessorNoAttrs(t *testing.T) {
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

func TestProcessorAddLabels(t *testing.T) {
	m := newMultiTest(
		t,
		NewFactory().CreateDefaultConfig(),
		nil,
	)

	tests := map[string]map[string]string{
		"1": {
			"pod":         "test-2323",
			"ns":          "default",
			"another tag": "value",
		},
		"2": {
			"pod": "test-12",
		},
	}
	for ip, attrs := range tests {
		m.kubernetesProcessorOperation(func(kp *kubernetesprocessor) {
			kp.kc.(*fakeClient).Pods[ip] = &kube.Pod{Attributes: attrs}
		})
	}

	var i int
	for ip, attrs := range tests {
		ctx := client.NewContext(context.Background(), &client.Client{IP: ip})
		m.testConsume(
			ctx,
			generateTraces(),
			generateMetrics(),
			generateLogs(),
			func(err error) {
				assert.NoError(t, err)
			})

		m.assertBatchesLen(i + 1)
		m.assertResourceObjectLen(i, 1)
		m.assertResource(i, 0, func(res pdata.Resource) {
			require.False(t, res.IsNil())
			assertResourceHasStringAttribute(t, res, "k8s.pod.ip", ip)
			for k, v := range attrs {
				assertResourceHasStringAttribute(t, res, k, v)
			}
		})

		i++
	}
}

func TestProcessorPicksUpPassthoughPodIp(t *testing.T) {
	m := newMultiTest(
		t,
		NewFactory().CreateDefaultConfig(),
		nil,
	)

	m.kubernetesProcessorOperation(func(kp *kubernetesprocessor) {
		kp.kc.(*fakeClient).Pods["2.2.2.2"] = &kube.Pod{
			Name: "PodA",
			Attributes: map[string]string{
				"k": "v",
				"1": "2",
			},
		}
	})

	m.testConsume(
		context.Background(),
		generateTraces(withPassthroughIP("2.2.2.2")),
		generateMetrics(withPassthroughIP("2.2.2.2")),
		generateLogs(withPassthroughIP("2.2.2.2")),
		func(err error) {
			assert.NoError(t, err)
		})

	m.assertBatchesLen(1)
	m.assertResourceObjectLen(0, 1)
	m.assertResourceAttributesLen(0, 0, 3)

	m.assertResource(0, 0, func(res pdata.Resource) {
		assertResourceHasStringAttribute(t, res, k8sIPLabelName, "2.2.2.2")
		assertResourceHasStringAttribute(t, res, "k", "v")
		assertResourceHasStringAttribute(t, res, "1", "2")
	})
}

func TestMetricsProcessorHostname(t *testing.T) {
	next := new(consumertest.MetricsSink)
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
	kc.Pods["3.3.3.3"] = &kube.Pod{
		Name: "PodA",
		Attributes: map[string]string{
			"kk": "vv",
		},
	}

	type testCase struct {
		name, hostname string
		expectedAttrs  map[string]string
	}

	testCases := []testCase{
		{
			name:     "invalid IP in hostname",
			hostname: "invalid-ip",
			expectedAttrs: map[string]string{
				conventions.AttributeHostHostname: "invalid-ip",
			},
		},
		{
			name:     "valid IP in hostname",
			hostname: "3.3.3.3",
			expectedAttrs: map[string]string{
				conventions.AttributeHostHostname: "3.3.3.3",
				k8sIPLabelName:                    "3.3.3.3",
				"kk":                              "vv",
			},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := generateMetrics(withHostname(tc.hostname))
			assert.NoError(t, p.ConsumeMetrics(context.Background(), metrics))
			require.Len(t, next.AllMetrics(), i+1)

			md := next.AllMetrics()[i]
			require.Equal(t, 1, md.ResourceMetrics().Len())
			res := md.ResourceMetrics().At(0).Resource()
			assert.Equal(t, len(tc.expectedAttrs), res.Attributes().Len())
			for k, v := range tc.expectedAttrs {
				assertResourceHasStringAttribute(t, res, k, v)
			}
		})
	}

}

func TestPassthroughStart(t *testing.T) {
	next := new(consumertest.TracesSink)
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
		consumertest.NewTracesNop(),
	)
	assert.NoError(t, err)
	caps := p.GetCapabilities()
	assert.True(t, caps.MutatesConsumedData)
}

func TestStartStop(t *testing.T) {
	var kp *kubernetesprocessor
	p, err := newTraceProcessor(
		NewFactory().CreateDefaultConfig(),
		consumertest.NewTracesNop(),
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

func assertResourceHasStringAttribute(t *testing.T, r pdata.Resource, k, v string) {
	got, ok := r.Attributes().Get(k)
	assert.True(t, ok, fmt.Sprintf("resource does not contain attribute %s", k))
	assert.EqualValues(t, pdata.AttributeValueSTRING, got.Type(), "attribute %s is not of type string", k)
	assert.EqualValues(t, v, got.StringVal(), "attribute %s is not equal to %s", k, v)
}
