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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func ptr[T any](v T) *T { return &v }

// Optimized: PartialObjectMetadata (ObjectMeta + OwnerRefs only)
func genPOMReplicaSets(n int) []runtime.Object {
	objs := make([]runtime.Object, 0, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("deploy-%d-abc123defg", i)
		pom := &meta_v1.PartialObjectMetadata{
			TypeMeta: meta_v1.TypeMeta{APIVersion: "apps/v1", Kind: "ReplicaSet"},
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				UID:       types.UID(fmt.Sprintf("uid-%d", i)),
				OwnerReferences: []meta_v1.OwnerReference{
					{APIVersion: "apps/v1", Kind: "Deployment", Name: fmt.Sprintf("deploy-%d", i), UID: types.UID(fmt.Sprintf("depuid-%d", i)), Controller: ptr(true)},
				},
				Labels: map[string]string{"app": "example"},
			},
		}
		objs = append(objs, pom)
	}
	return objs
}

func setReplicaSetListAndWatch(fc *fake.Clientset, initial []runtime.Object) {
	fc.Fake.PrependReactor("list", "replicasets", func(_ ktesting.Action) (bool, runtime.Object, error) {
		list := &apps_v1.ReplicaSetList{}
		for _, obj := range initial {
			list.Items = append(list.Items, *(obj.(*apps_v1.ReplicaSet)).DeepCopy())
		}
		return true, list, nil
	})
	fc.Fake.PrependWatchReactor("replicasets", func(_ ktesting.Action) (bool, watch.Interface, error) {
		return true, watch.NewFake(), nil
	})
}

func startAndSync(b *testing.B, c Client) {
	if err := c.Start(); err != nil {
		b.Fatalf("start: %v", err)
	}
	_ = wait.PollUntilContextTimeout(context.Background(), time.Millisecond, 50*time.Millisecond, true, func(context.Context) (bool, error) { return true, nil })
	defer c.Stop()
}

// Partial metadata for a parameterized N
func runPartialMetadata(b *testing.B, N int) {
	metaOnly := genPOMReplicaSets(N)
	for i := 0; i < b.N; i++ {
		fc := fake.NewClientset()
		// We still attach the typed reactors so the fake client satisfies list/watch,
		// but the informer provider below will emit PartialObjectMetadata.
		setReplicaSetListAndWatch(fc, metaOnly)

		newClientSet := func(_ k8sconfig.APIConfig) (kubernetes.Interface, error) { return fc, nil }
		rules := ExtractionRules{DeploymentName: true}
		filters := Filters{}

		factory := InformersFactoryList{
			newInformer:           newSharedInformer,
			newNamespaceInformer:  NewNoOpInformer,
			newReplicaSetInformer: newReplicaSetMetaInformer(),
		}

		set := componenttest.NewNopTelemetrySettings()
		c, err := New(set, k8sconfig.APIConfig{}, rules, filters, []Association{}, Excludes{}, newClientSet, factory, false, 0)
		if err != nil {
			b.Fatalf("New: %v", err)
		}

		b.ReportAllocs()
		startAndSync(b, c)
	}
}

// Benchmark suite to check increasing amount of replicasets
// gather results using
// go test -run ^$ -bench RS_ResourceSweep_InProcess -benchmem -count=6 -benchtime=1x > sweep.txt
// benchstat sweep.txt
func Benchmark_RS_ResourceSweep_InProcess(b *testing.B) {
	Ns := []int{5000, 10000, 20000, 200000}

	for _, N := range Ns {
		b.Run(fmt.Sprintf("Partial_Metadata_N=%d", N), func(b *testing.B) {
			runPartialMetadata(b, N)
		})
	}
}
