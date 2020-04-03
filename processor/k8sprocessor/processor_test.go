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
	"testing"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	"github.com/open-telemetry/opentelemetry-collector/client"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exportertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

func TestNewTraceProcessor(t *testing.T) {
	_, err := NewTraceProcessor(
		zap.NewNop(),
		exportertest.NewNopTraceExporterOld(),
		kube.NewFakeClient,
	)
	require.NoError(t, err)
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
	err = kp.ConsumeTraceData(ctx, consumerdata.TraceData{})
	require.NoError(t, err)

	require.Len(t, next.data, 1)
	require.NotNil(t, next.data[0].Resource)
	assert.Equal(t, next.data[0].Resource.Labels["ip"], "1.1.1.1")
}

func TestNoIP(t *testing.T) {
	next := &testConsumer{}
	kp, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		kube.NewFakeClient,
	)
	require.NoError(t, err)

	err = kp.ConsumeTraceData(context.Background(), consumerdata.TraceData{})
	require.NoError(t, err)

	require.Len(t, next.data, 1)
	assert.Nil(t, next.data[0].Resource)
}

func TestJaegerIP(t *testing.T) {
	next := &testConsumer{}
	kp, err := NewTraceProcessor(
		zap.NewNop(),
		next,
		kube.NewFakeClient,
	)
	require.NoError(t, err)

	// should not detect IP
	err = kp.ConsumeTraceData(
		context.Background(),
		consumerdata.TraceData{
			Node: &commonpb.Node{
				Attributes: map[string]string{
					"ip": "1.1.1.1",
				},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, next.data, 1)
	assert.Nil(t, next.data[0].Resource)

	// should detect IP from Node when source format is "jaeger"
	err = kp.ConsumeTraceData(
		context.Background(),
		consumerdata.TraceData{
			SourceFormat: "jaeger",
			Node: &commonpb.Node{
				Attributes: map[string]string{
					"ip": "1.1.1.1",
				},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, next.data, 2)
	assert.NotNil(t, next.data[1].Resource)
	assert.Equal(t, next.data[1].Resource.Labels["ip"], "1.1.1.1")
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
		err = p.ConsumeTraceData(ctx, consumerdata.TraceData{})
		require.NoError(t, err)

		require.Len(t, next.data, i+1)
		td := next.data[i]
		require.NotNil(t, td.Resource)
		assert.Equal(t, td.Resource.Labels["ip"], ip)
		for k, v := range attrs {
			vv, ok := td.Resource.Labels[k]
			assert.True(t, ok)
			assert.Equal(t, v, vv)
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
	p.Start(component.NewMockHost())
	p.Shutdown()
}

func fakeClientFromProcessor(t *testing.T, p component.TraceProcessorOld) *kube.FakeClient {
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
	data []consumerdata.TraceData
}

func (ts *testConsumer) ConsumeTraceData(ctx context.Context, td consumerdata.TraceData) error {
	ts.data = append(ts.data, td)
	return nil
}
