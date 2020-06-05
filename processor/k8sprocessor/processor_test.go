// Copyright 2019 Omnition Authors
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

func TestNewTraceProcessor(t *testing.T) {
	_, err := NewTraceProcessor(
		zap.NewNop(),
		exportertest.NewNopTraceExporter(),
		kube.NewFakeClient,
	)
	require.NoError(t, err)
}

func TestBadOption(t *testing.T) {
	opt := func(p *kubernetesprocessor) error {
		return fmt.Errorf("bad option")
	}
	p, err := NewTraceProcessor(
		zap.NewNop(),
		exportertest.NewNopTraceExporter(),
		kube.NewFakeClient,
		opt,
	)
	assert.Nil(t, p)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "bad option")
}

func TestBadClientProvider(t *testing.T) {
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
	next := &testConsumer{}
	kp, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		kube.NewFakeClient,
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
	next := &testConsumer{}
	kp, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		kube.NewFakeClient,
	)
	require.NoError(t, err)

	err = kp.ConsumeTraces(context.Background(), pdata.NewTraces())
	require.NoError(t, err)
	require.Len(t, next.data, 1)
}

func TestNoIP(t *testing.T) {
	next := &testConsumer{}
	kp, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		kube.NewFakeClient,
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
	next := &testConsumer{}
	kp, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		kube.NewFakeClient,
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

func TestAddLabels(t *testing.T) {
	next := &testConsumer{}
	p, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		kube.NewFakeClient,
	)
	require.NoError(t, err)

	kc := fakeClientFromProcessor(t, p)

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
	next := &testConsumer{}
	opts := []Option{WithPassthrough()}

	p, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		kube.NewFakeClient,
		opts...,
	)
	require.NoError(t, err)

	// Just make sure this doesn't fail when Passthrough is enabled
	assert.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, p.Shutdown(context.Background()))
}

func TestStartStop(t *testing.T) {
	next := &testConsumer{}
	p, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		kube.NewFakeClient,
	)
	require.NoError(t, err)

	assert.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))

	pr := p.(*kubernetesprocessor)
	client := pr.kc.(*kube.FakeClient)
	controller := client.Informer.GetController().(*kube.FakeController)

	assert.False(t, controller.HasStopped())
	assert.NoError(t, p.Shutdown(context.Background()))
	time.Sleep(time.Millisecond * 500)
	assert.True(t, controller.HasStopped())
}

func fakeClientFromProcessor(t *testing.T, p component.TraceProcessor) *kube.FakeClient {
	kp, ok := p.(*kubernetesprocessor)
	if !ok {
		assert.FailNow(t, "could not assert processor %s to kubernetesprocessor", p)
		return nil
	}
	kc, ok := kp.kc.(*kube.FakeClient)
	if !ok {
		assert.FailNow(t, "could not assert kube client %s to kube.FakeClient", p)
		return nil

	}
	return kc
}

type testConsumer struct {
	data []pdata.Traces
}

func (ts *testConsumer) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	ts.data = append(ts.data, td)
	return nil
}

func assertResourceHasStringAttribute(t *testing.T, r pdata.Resource, k, v string) {
	got, ok := r.Attributes().Get(k)
	assert.True(t, ok, fmt.Sprintf("resource does not contain attribute %s", k))
	assert.Equal(t, pdata.AttributeValueSTRING, got.Type(), "attribute %s is not of type string", k)
	assert.Equal(t, v, got.StringVal(), "attribute %s is not equal to %s", k, v)
}
