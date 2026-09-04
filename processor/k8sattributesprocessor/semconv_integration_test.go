// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package k8sattributesprocessor

// These tests validate the resource attributes that the processor adds
// against the OpenTelemetry semantic conventions, using a Weaver live-check
// container via pkg/semconvtest. By default the check runs against the
// latest published semantic-conventions registry, which Weaver downloads at
// startup, on the otel/weaver:latest image. Both can be pinned with
// semconvtest.WithVersion and semconvtest.WithRegistry when deterministic
// results are needed.

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/metadata"
)

// TestSemconvCompliancePodAttributes checks the pod resource attributes that
// the processor adds against semantic conventions. The input metric is fully
// semconv-compliant on its own, so any violation that the live-check reports
// must come from an attribute the processor added.
func TestSemconvCompliancePodAttributes(t *testing.T) {
	next := new(consumertest.MetricsSink)
	var kp *kubernetesprocessor
	p, err := newMetricsProcessor(
		NewFactory().CreateDefaultConfig(),
		next,
		withExtractKubernetesProcessorInto(&kp),
	)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, p.Shutdown(t.Context())) }()

	kc := kp.kc.(*fakeClient)
	kc.Pods[newPodIdentifier("connection", "k8s.pod.ip", "1.1.1.1")] = &kube.Pod{
		Name: "test-pod",
		Attributes: map[string]string{
			"k8s.pod.name":        "test-pod",
			"k8s.pod.uid":         "ef10d10b-2da5-4030-812e-5f45c1531227",
			"k8s.pod.start_time":  time.Now().Format(time.RFC3339),
			"k8s.namespace.name":  "default",
			"k8s.node.name":       "node-1",
			"k8s.deployment.name": "test-deployment",
		},
	}

	ctx := client.NewContext(t.Context(), client.Info{
		Addr: &net.IPAddr{IP: net.ParseIP("1.1.1.1")},
	})
	require.NoError(t, p.ConsumeMetrics(ctx, generateSemconvCarrierMetrics()))
	require.Len(t, next.AllMetrics(), 1, "expected one batch of metrics")

	enriched := next.AllMetrics()[0]
	res := enriched.ResourceMetrics().At(0).Resource()
	_, ok := res.Attributes().Get("k8s.pod.name")
	require.True(t, ok, "expected the processor to add k8s.pod.name")

	semconvtest.TestMetrics(t, enriched)
}

// TestSemconvComplianceContainerAttributesV1 checks the container attributes
// that the processor adds when both feature gates enable the stable (v1)
// semantic conventions. This is the output that the v1 migration targets.
func TestSemconvComplianceContainerAttributesV1(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), true))
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.ID(), true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.ID(), false))
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.ID(), false))
	}()

	next := new(consumertest.MetricsSink)
	var kp *kubernetesprocessor
	p, err := newMetricsProcessor(
		NewFactory().CreateDefaultConfig(),
		next,
		withExtractKubernetesProcessorInto(&kp),
	)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, p.Shutdown(t.Context())) }()

	kc := kp.kc.(*fakeClient)
	kc.Pods[newPodIdentifier("connection", "k8s.pod.ip", "1.1.1.1")] = &kube.Pod{
		Name: "test-pod",
		Attributes: map[string]string{
			"k8s.pod.name":       "test-pod",
			"k8s.namespace.name": "default",
		},
		Containers: kube.PodContainers{
			ByName: map[string]*kube.Container{
				"app": {
					Name:      "app",
					ImageName: "nginx",
					ImageTags: []string{"1.25.3"},
				},
			},
		},
	}

	ctx := client.NewContext(t.Context(), client.Info{
		Addr: &net.IPAddr{IP: net.ParseIP("1.1.1.1")},
	})
	require.NoError(t, p.ConsumeMetrics(ctx, generateSemconvCarrierMetrics()))
	require.Len(t, next.AllMetrics(), 1, "expected one batch of metrics")

	enriched := next.AllMetrics()[0]
	res := enriched.ResourceMetrics().At(0).Resource()
	_, ok := res.Attributes().Get("container.image.tags")
	require.True(t, ok, "expected the processor to add container.image.tags")

	semconvtest.TestMetrics(t, enriched)
}

// generateSemconvCarrierMetrics builds a semconv-compliant
// http.server.request.duration histogram. This input produces no violations
// on its own, so the live-check result reflects only the attributes that the
// processor adds.
// The metric name, the unit, and the attribute names below are hard-coded to
// the stable HTTP semantic conventions. If a live-check failure mentions
// http.*, url.* or network.* attributes, check whether these labels still
// match the current registry.
func generateSemconvCarrierMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()

	rm.Resource().Attributes().PutStr("service.name", "sample-http-server")
	rm.Resource().Attributes().PutStr("service.version", "1.0.0")

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor")
	sm.Scope().SetVersion("0.1.0")

	m := sm.Metrics().AppendEmpty()
	m.SetName("http.server.request.duration")
	m.SetUnit("s")

	h := m.SetEmptyHistogram()
	h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	now := time.Now()
	dp := h.DataPoints().AppendEmpty()
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-time.Minute)))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
	dp.SetCount(10)
	dp.SetSum(0.5)
	dp.ExplicitBounds().FromRaw([]float64{0.005, 0.01, 0.025, 0.05, 0.1})
	dp.BucketCounts().FromRaw([]uint64{0, 2, 4, 3, 1, 0})

	dp.Attributes().PutStr("http.request.method", "GET")
	dp.Attributes().PutInt("http.response.status_code", 200)
	dp.Attributes().PutStr("http.route", "/api/users")
	dp.Attributes().PutStr("url.scheme", "https")
	dp.Attributes().PutStr("network.protocol.version", "1.1")

	return md
}
