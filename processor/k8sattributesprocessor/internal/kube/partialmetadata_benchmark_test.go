package kube

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"go.opentelemetry.io/collector/component/componenttest"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func ptr[T any](v T) *T { return &v }

// Heavy payload for baseline typed decode
func heavyPodTemplateSpec() core_v1.PodTemplateSpec {
	envs := make([]core_v1.EnvVar, 0, 20)
	for i := 0; i < 20; i++ {
		envs = append(envs, core_v1.EnvVar{Name: fmt.Sprintf("ENV_%d", i), Value: "VALUE"})
	}
	containers := make([]core_v1.Container, 0, 6)
	for i := 0; i < 6; i++ {
		containers = append(containers, core_v1.Container{
			Name:  fmt.Sprintf("c%d", i),
			Image: fmt.Sprintf("repo/app:v%d", i),
			Env:   envs,
		})
	}
	return core_v1.PodTemplateSpec{
		ObjectMeta: meta_v1.ObjectMeta{Labels: map[string]string{"app": "example"}},
		Spec:       core_v1.PodSpec{Containers: containers},
	}
}

// Baseline: full ReplicaSets
func genFullReplicaSets(n int) []runtime.Object {
	objs := make([]runtime.Object, 0, n)
	tpl := heavyPodTemplateSpec()
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("deploy-%d-abc123defg", i)
		rs := &apps_v1.ReplicaSet{
			TypeMeta: meta_v1.TypeMeta{APIVersion: "apps/v1", Kind: "ReplicaSet"},
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				UID:       types.UID(fmt.Sprintf("uid-%d", i)),
				OwnerReferences: []meta_v1.OwnerReference{
					{APIVersion: "apps/v1", Kind: "Deployment", Name: fmt.Sprintf("deploy-%d", i), UID: types.UID(fmt.Sprintf("depuid-%d", i)), Controller: ptr(true)},
				},
				Labels:      map[string]string{"app": "example"},
				Annotations: map[string]string{"note": "heavy payload"},
			},
			Spec: apps_v1.ReplicaSetSpec{
				Replicas: ptr(int32(3)),
				Template: tpl,
			},
		}
		objs = append(objs, rs)
	}
	return objs
}

// Optimized: metadata-like minimal ReplicaSets (ObjectMeta + OwnerRefs only)
func genMetaOnlyReplicaSets(n int) []runtime.Object {
	objs := make([]runtime.Object, 0, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("deploy-%d-abc123defg", i)
		rs := &apps_v1.ReplicaSet{
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
		objs = append(objs, rs)
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

// Baseline: typed informer + full objects (works on main and branch)
func Benchmark_RS_Typed_Full_InProcess(b *testing.B) {
	const N = 16700
	initial := genFullReplicaSets(N)

	for i := 0; i < b.N; i++ {
		fc := fake.NewSimpleClientset()
		setReplicaSetListAndWatch(fc, initial)

		newClientSet := func(_ k8sconfig.APIConfig) (kubernetes.Interface, error) { return fc, nil }
		rules := ExtractionRules{DeploymentName: true}
		filters := Filters{}

		factory := InformersFactoryList{
			newInformer:           newSharedInformer,
			newNamespaceInformer:  NewNoOpInformer,
			newReplicaSetInformer: nil, // default typed path
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

// Optimized: meta-like provider + minimal payload (works on main and branch)
func NewReplicaSetMetaInformerProviderForBench(initial func() *apps_v1.ReplicaSetList) func(kubernetes.Interface, string) cache.SharedInformer {
	return func(_ kubernetes.Interface, _ string) cache.SharedInformer {
		lw := &cache.ListWatch{
			ListFunc: func(opts meta_v1.ListOptions) (runtime.Object, error) {
				return initial().DeepCopyObject(), nil
			},
			WatchFunc: func(opts meta_v1.ListOptions) (watch.Interface, error) {
				return watch.NewFake(), nil
			},
		}
		inf := cache.NewSharedIndexInformer(
			lw,
			&apps_v1.ReplicaSet{},
			0,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		)
		_ = inf.SetTransform(func(object any) (any, error) {
			if rs, ok := object.(*apps_v1.ReplicaSet); ok {
				return removeUnnecessaryReplicaSetData(rs), nil
			}
			return object, nil
		})
		return inf
	}
}

func Benchmark_RS_MetaLike_InProcess(b *testing.B) {
	const N = 16700
	metaOnly := genMetaOnlyReplicaSets(N)
	toList := func() *apps_v1.ReplicaSetList {
		lst := &apps_v1.ReplicaSetList{Items: make([]apps_v1.ReplicaSet, 0, len(metaOnly))}
		for _, obj := range metaOnly {
			lst.Items = append(lst.Items, *(obj.(*apps_v1.ReplicaSet)).DeepCopy())
		}
		return lst
	}

	for i := 0; i < b.N; i++ {
		fc := fake.NewSimpleClientset()
		newClientSet := func(_ k8sconfig.APIConfig) (kubernetes.Interface, error) { return fc, nil }
		rules := ExtractionRules{DeploymentName: true}
		filters := Filters{}

		factory := InformersFactoryList{
			newInformer:           newSharedInformer,
			newNamespaceInformer:  NewNoOpInformer,
			newReplicaSetInformer: NewReplicaSetMetaInformerProviderForBench(toList),
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
