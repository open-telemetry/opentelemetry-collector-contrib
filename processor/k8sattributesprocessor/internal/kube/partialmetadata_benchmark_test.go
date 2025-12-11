package kube

import (
	"context"
	"fmt"
	"testing"
	"time"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"go.opentelemetry.io/collector/component/componenttest"
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

// Typed baseline for a parameterized N
func runTypedBaseline(b *testing.B, N int) {
	initial := genFullReplicaSets(N)
	for i := 0; i < b.N; i++ {
		fc := fake.NewClientset()
		setReplicaSetListAndWatch(fc, initial)

		newClientSet := func(_ k8sconfig.APIConfig) (kubernetes.Interface, error) { return fc, nil }
		rules := ExtractionRules{DeploymentName: true}
		filters := Filters{}

		factory := InformersFactoryList{
			newInformer:           newSharedInformer,
			newNamespaceInformer:  NewNoOpInformer,
			newReplicaSetInformer: newReplicaSetSharedInformer,
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
			newReplicaSetInformer: NewReplicaSetMetaInformerProvider(k8sconfig.APIConfig{}),
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

// Benchmark suite: sweep N values and run typed vs partial
func Benchmark_RS_ResourceSweep_InProcess(b *testing.B) {
	Ns := []int{1000, 5000, 10000, 20000}

	for _, N := range Ns {
		b.Run(fmt.Sprintf("Typed_Full_N=%d", N), func(b *testing.B) {
			runTypedBaseline(b, N)
		})
		b.Run(fmt.Sprintf("Partial_Metadata_N=%d", N), func(b *testing.B) {
			runPartialMetadata(b, N)
		})
	}
}
