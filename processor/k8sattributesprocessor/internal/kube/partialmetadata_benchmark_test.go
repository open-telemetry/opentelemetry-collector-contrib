// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.opentelemetry.io/collector/component/componenttest"
	apps_v1 "k8s.io/api/apps/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func ptr[T any](v T) *T { return &v }

func genTypedRS(n int) []*apps_v1.ReplicaSet {
	out := make([]*apps_v1.ReplicaSet, n)
	for i := range out {
		name := fmt.Sprintf("deploy-%d-abc123defg", i)
		out[i] = &apps_v1.ReplicaSet{
			TypeMeta: meta_v1.TypeMeta{APIVersion: "apps/v1", Kind: "ReplicaSet"},
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				UID:       types.UID(fmt.Sprintf("uid-%d", i)),
				OwnerReferences: []meta_v1.OwnerReference{
					{APIVersion: "apps/v1", Kind: "Deployment", Name: fmt.Sprintf("deploy-%d", i), UID: types.UID(fmt.Sprintf("depuid-%d", i)), Controller: ptr(true)},
				},
			},
		}
	}
	return out
}

func prebuiltRSList(rs []*apps_v1.ReplicaSet) *apps_v1.ReplicaSetList {
	list := &apps_v1.ReplicaSetList{Items: make([]apps_v1.ReplicaSet, len(rs))}
	for i := range rs {
		list.Items[i] = *rs[i]
	}
	return list
}

func typedClientWithList(list *apps_v1.ReplicaSetList) *fake.Clientset {
	fc := fake.NewClientset()
	fc.PrependReactor("list", "replicasets", func(_ ktesting.Action) (bool, runtime.Object, error) {
		return true, list, nil
	})
	fc.PrependWatchReactor("replicasets", func(_ ktesting.Action) (bool, watch.Interface, error) {
		return true, watch.NewFake(), nil
	})
	return fc
}

func Benchmark_RS_ResourceSweep_InProcess(b *testing.B) {
	ns := []int{10000, 200000}

	for _, n := range ns {
		b.Run(fmt.Sprintf("Typed_RS_N=%d", n), func(b *testing.B) {
			rs := genTypedRS(n)
			list := prebuiltRSList(rs)

			rules := ExtractionRules{
				DeploymentName: true,
				// Optional: avoid starting pod/node/namespace informers
				PodName: false, Namespace: false, Node: false,
			}
			filters := Filters{}
			set := componenttest.NewNopTelemetrySettings()

			for i := 0; i < b.N; i++ {
				b.StopTimer()

				fc := typedClientWithList(list)
				factory := InformersFactoryList{
					newInformer:           NewFakeInformer,
					newNamespaceInformer:  NewNoOpInformer,
					newReplicaSetInformer: newReplicaSetSharedInformer,
				}
				newClientSet := func(_ k8sconfig.APIConfig) (k8sconfig.ClientBundle, error) {
					return k8sconfig.ClientBundle{K8s: fc}, nil
				}

				c, err := New(set, k8sconfig.APIConfig{}, rules, filters, nil, Excludes{}, newClientSet, factory, false, 0)
				if err != nil {
					b.Fatalf("New: %v", err)
				}

				b.StartTimer()

				if err := c.Start(); err != nil {
					b.Fatalf("start: %v", err)
				}
				_ = wait.PollUntilContextTimeout(
					b.Context(), time.Millisecond, 5*time.Second, true,
					func(context.Context) (bool, error) { return true, nil },
				)

				b.StopTimer()
				c.Stop()
				b.StartTimer()
			}
		})
	}
}
