// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	api_v1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func newBenchmarkKubeletClient(tb testing.TB, filters Filters) *WatchClient {
	tb.Helper()

	exclude := Excludes{
		Pods: []ExcludePods{
			{Name: regexp.MustCompile(`jaeger-agent`)},
			{Name: regexp.MustCompile(`jaeger-collector`)},
		},
	}
	associations := []Association{
		{Sources: []AssociationSource{{From: ConnectionSource}}},
		{Sources: []AssociationSource{{From: ResourceSource, Name: "k8s.pod.uid"}}},
	}
	factory := InformersFactoryList{
		newInformer:           NewFakeInformer,
		newNamespaceInformer:  NewFakeNamespaceInformer,
		newReplicaSetInformer: NewFakeReplicaSetInformer,
	}
	rules := ExtractionRules{
		Namespace: true,
		Node:      true,
		PodName:   true,
		PodUID:    true,
		StartTime: true,
	}

	c, err := New(
		componenttest.NewNopTelemetrySettings(),
		k8sconfig.APIConfig{},
		rules,
		filters,
		associations,
		exclude,
		newFakeAPIClientset,
		factory,
		false,
		10*time.Second,
		0,
		120*time.Second,
		KubeletConfig{},
	)
	require.NoError(tb, err)

	wc := c.(*WatchClient)
	wc.kubeletPods = map[string]*api_v1.Pod{}
	return wc
}

func makeBenchmarkKubeletPods(n int, uidOffset int) []api_v1.Pod {
	pods := make([]api_v1.Pod, 0, n)
	for i := range n {
		idx := uidOffset + i
		pods = append(pods, kubeletTestPod(
			fmt.Sprintf("uid-%d", idx),
			fmt.Sprintf("pod-%d", idx),
			"default",
			"node-a",
			fmt.Sprintf("10.0.%d.%d", idx/250, (idx%250)+1),
			map[string]string{"app": "bench"},
		))
	}
	return pods
}

func BenchmarkKubeletReconcilePods(b *testing.B) {
	for _, n := range []int{1000, 5000, 10000} {
		b.Run(fmt.Sprintf("steady_state_%d", n), func(b *testing.B) {
			c := newBenchmarkKubeletClient(b, Filters{Node: "node-a"})
			pods := makeBenchmarkKubeletPods(n, 0)
			c.reconcileKubeletPods(pods)

			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				c.reconcileKubeletPods(pods)
			}
		})
	}
}

func BenchmarkKubeletReconcilePodsWithChurn(b *testing.B) {
	for _, n := range []int{1000, 5000, 10000} {
		b.Run(fmt.Sprintf("ten_percent_churn_%d", n), func(b *testing.B) {
			c := newBenchmarkKubeletClient(b, Filters{Node: "node-a"})
			base := makeBenchmarkKubeletPods(n, 0)
			churned := append([]api_v1.Pod{}, base...)
			churn := n / 10
			replacement := makeBenchmarkKubeletPods(churn, n)
			copy(churned[n-churn:], replacement)
			c.reconcileKubeletPods(base)

			b.ResetTimer()
			b.ReportAllocs()
			useBase := false
			for b.Loop() {
				if useBase {
					c.reconcileKubeletPods(base)
				} else {
					c.reconcileKubeletPods(churned)
				}
				useBase = !useBase
			}
		})
	}
}
