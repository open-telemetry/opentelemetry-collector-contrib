// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

type jobInstanceDefinition struct {
	job, instance, host, scheme, port string
}

type k8sResourceDefinition struct {
	podName, podUID, container, node, rs, ds, ss, job, cronjob, ns string
}

func makeK8sResource(jobInstance *jobInstanceDefinition, def *k8sResourceDefinition) pcommon.Resource {
	resource := makeResourceWithJobInstanceScheme(jobInstance, true)
	attrs := resource.Attributes()
	if def.podName != "" {
		attrs.PutStr(string(conventions.K8SPodNameKey), def.podName)
	}
	if def.podUID != "" {
		attrs.PutStr(string(conventions.K8SPodUIDKey), def.podUID)
	}
	if def.container != "" {
		attrs.PutStr(string(conventions.K8SContainerNameKey), def.container)
	}
	if def.node != "" {
		attrs.PutStr(string(conventions.K8SNodeNameKey), def.node)
	}
	if def.rs != "" {
		attrs.PutStr(string(conventions.K8SReplicaSetNameKey), def.rs)
	}
	if def.ds != "" {
		attrs.PutStr(string(conventions.K8SDaemonSetNameKey), def.ds)
	}
	if def.ss != "" {
		attrs.PutStr(string(conventions.K8SStatefulSetNameKey), def.ss)
	}
	if def.job != "" {
		attrs.PutStr(string(conventions.K8SJobNameKey), def.job)
	}
	if def.cronjob != "" {
		attrs.PutStr(string(conventions.K8SCronJobNameKey), def.cronjob)
	}
	if def.ns != "" {
		attrs.PutStr(string(conventions.K8SNamespaceNameKey), def.ns)
	}
	return resource
}

func makeResourceWithJobInstanceScheme(def *jobInstanceDefinition, hasHost bool) pcommon.Resource {
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	// Using hardcoded values to assert on outward expectations so that
	// when variables change, these tests will fail and we'll have reports.
	attrs.PutStr("service.name", def.job)
	if hasHost {
		attrs.PutStr("server.address", def.host)
	}
	attrs.PutStr("service.instance.id", def.instance)
	attrs.PutStr("server.port", def.port)
	attrs.PutStr("url.scheme", def.scheme)
	return resource
}

func makeResourceWithJobInstanceSchemeDuplicate(def *jobInstanceDefinition, hasHost bool) pcommon.Resource {
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	// Using hardcoded values to assert on outward expectations so that
	// when variables change, these tests will fail and we'll have reports.
	attrs.PutStr("service.name", def.job)
	if hasHost {
		attrs.PutStr("net.host.name", def.host)
		attrs.PutStr("server.address", def.host)
	}
	attrs.PutStr("service.instance.id", def.instance)
	attrs.PutStr("net.host.port", def.port)
	attrs.PutStr("http.scheme", def.scheme)
	attrs.PutStr("server.port", def.port)
	attrs.PutStr("url.scheme", def.scheme)
	return resource
}

func TestCreateNodeAndResourcePromToOTLP(t *testing.T) {
	tests := []struct {
		name, job                   string
		instance                    string
		sdLabels                    labels.Labels
		removeOldSemconvFeatureGate bool
		want                        pcommon.Resource
	}{
		{
			name: "all attributes proper",
			job:  "job", instance: "hostname:8888", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			removeOldSemconvFeatureGate: true,
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
				"job", "hostname:8888", "hostname", "http", "8888",
			}, true),
		},
		{
			name: "missing port",
			job:  "job", instance: "myinstance", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "https"}),
			removeOldSemconvFeatureGate: true,
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
				"job", "myinstance", "myinstance", "https", "",
			}, true),
		},
		{
			name: "blank scheme",
			job:  "job", instance: "myinstance:443", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: ""}),
			removeOldSemconvFeatureGate: true,
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
				"job", "myinstance:443", "myinstance", "", "443",
			}, true),
		},
		{
			name: "blank instance, blank scheme",
			job:  "job", instance: "", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: ""}),
			removeOldSemconvFeatureGate: true,
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
				"job", "", "", "", "",
			}, true),
		},
		{
			name: "blank instance, non-blank scheme",
			job:  "job", instance: "", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			removeOldSemconvFeatureGate: true,
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
				"job", "", "", "http", "",
			}, true),
		},
		{
			name: "0.0.0.0 address",
			job:  "job", instance: "0.0.0.0:8888", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			removeOldSemconvFeatureGate: true,
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
				"job", "0.0.0.0:8888", "", "http", "8888",
			}, false),
		},
		{
			name: "localhost",
			job:  "job", instance: "localhost:8888", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			removeOldSemconvFeatureGate: true,
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
				"job", "localhost:8888", "", "http", "8888",
			}, false),
		},
		{
			name: "all attributes proper with duplicates",
			job:  "job", instance: "hostname:8888", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: makeResourceWithJobInstanceSchemeDuplicate(&jobInstanceDefinition{
				"job", "hostname:8888", "hostname", "http", "8888",
			}, true),
		},
		{
			name: "missing port with duplicates",
			job:  "job", instance: "myinstance", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "https"}),
			want: makeResourceWithJobInstanceSchemeDuplicate(&jobInstanceDefinition{
				"job", "myinstance", "myinstance", "https", "",
			}, true),
		},
		{
			name: "blank scheme with duplicates",
			job:  "job", instance: "myinstance:443", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: ""}),
			want: makeResourceWithJobInstanceSchemeDuplicate(&jobInstanceDefinition{
				"job", "myinstance:443", "myinstance", "", "443",
			}, true),
		},
		{
			name: "blank instance, blank scheme with duplicates",
			job:  "job", instance: "", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: ""}),
			want: makeResourceWithJobInstanceSchemeDuplicate(&jobInstanceDefinition{
				"job", "", "", "", "",
			}, true),
		},
		{
			name: "blank instance, non-blank scheme with duplicates",
			job:  "job", instance: "", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: makeResourceWithJobInstanceSchemeDuplicate(&jobInstanceDefinition{
				"job", "", "", "http", "",
			}, true),
		},
		{
			name: "0.0.0.0 address with duplicates",
			job:  "job", instance: "0.0.0.0:8888", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: makeResourceWithJobInstanceSchemeDuplicate(&jobInstanceDefinition{
				"job", "0.0.0.0:8888", "", "http", "8888",
			}, false),
		},
		{
			name: "localhost with duplicates",
			job:  "job", instance: "localhost:8888", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: makeResourceWithJobInstanceSchemeDuplicate(&jobInstanceDefinition{
				"job", "localhost:8888", "", "http", "8888",
			}, false),
		},
		{
			name: "kubernetes daemonset pod",
			job:  "job", instance: "hostname:8888", sdLabels: labels.New(
				labels.Label{Name: "__scheme__", Value: "http"},
				labels.Label{Name: "__meta_kubernetes_pod_name", Value: "my-pod-23491"},
				labels.Label{Name: "__meta_kubernetes_pod_uid", Value: "84279wretgu89dg489q2"},
				labels.Label{Name: "__meta_kubernetes_pod_container_name", Value: "my-container"},
				labels.Label{Name: "__meta_kubernetes_pod_node_name", Value: "k8s-node-123"},
				labels.Label{Name: "__meta_kubernetes_pod_controller_name", Value: "my-pod"},
				labels.Label{Name: "__meta_kubernetes_pod_controller_kind", Value: "DaemonSet"},
				labels.Label{Name: "__meta_kubernetes_namespace", Value: "kube-system"},
			),
			removeOldSemconvFeatureGate: true,
			want: makeK8sResource(&jobInstanceDefinition{
				"job", "hostname:8888", "hostname", "http", "8888",
			}, &k8sResourceDefinition{
				podName:   "my-pod-23491",
				podUID:    "84279wretgu89dg489q2",
				container: "my-container",
				node:      "k8s-node-123",
				ds:        "my-pod",
				ns:        "kube-system",
			}),
		},
		{
			name: "kubernetes replicaset pod",
			job:  "job", instance: "hostname:8888", sdLabels: labels.New(
				labels.Label{Name: "__scheme__", Value: "http"},
				labels.Label{Name: "__meta_kubernetes_pod_name", Value: "my-pod-23491"},
				labels.Label{Name: "__meta_kubernetes_pod_uid", Value: "84279wretgu89dg489q2"},
				labels.Label{Name: "__meta_kubernetes_pod_container_name", Value: "my-container"},
				labels.Label{Name: "__meta_kubernetes_pod_node_name", Value: "k8s-node-123"},
				labels.Label{Name: "__meta_kubernetes_pod_controller_name", Value: "my-pod"},
				labels.Label{Name: "__meta_kubernetes_pod_controller_kind", Value: "ReplicaSet"},
				labels.Label{Name: "__meta_kubernetes_namespace", Value: "kube-system"},
			),
			removeOldSemconvFeatureGate: true,
			want: makeK8sResource(&jobInstanceDefinition{
				"job", "hostname:8888", "hostname", "http", "8888",
			}, &k8sResourceDefinition{
				podName:   "my-pod-23491",
				podUID:    "84279wretgu89dg489q2",
				container: "my-container",
				node:      "k8s-node-123",
				rs:        "my-pod",
				ns:        "kube-system",
			}),
		},
		{
			name: "kubernetes statefulset pod",
			job:  "job", instance: "hostname:8888", sdLabels: labels.New(
				labels.Label{Name: "__scheme__", Value: "http"},
				labels.Label{Name: "__meta_kubernetes_pod_name", Value: "my-pod-23491"},
				labels.Label{Name: "__meta_kubernetes_pod_uid", Value: "84279wretgu89dg489q2"},
				labels.Label{Name: "__meta_kubernetes_pod_container_name", Value: "my-container"},
				labels.Label{Name: "__meta_kubernetes_pod_node_name", Value: "k8s-node-123"},
				labels.Label{Name: "__meta_kubernetes_pod_controller_name", Value: "my-pod"},
				labels.Label{Name: "__meta_kubernetes_pod_controller_kind", Value: "StatefulSet"},
				labels.Label{Name: "__meta_kubernetes_namespace", Value: "kube-system"},
			),
			removeOldSemconvFeatureGate: true,
			want: makeK8sResource(&jobInstanceDefinition{
				"job", "hostname:8888", "hostname", "http", "8888",
			}, &k8sResourceDefinition{
				podName:   "my-pod-23491",
				podUID:    "84279wretgu89dg489q2",
				container: "my-container",
				node:      "k8s-node-123",
				ss:        "my-pod",
				ns:        "kube-system",
			}),
		},
		{
			name: "kubernetes job pod",
			job:  "job", instance: "hostname:8888", sdLabels: labels.New(
				labels.Label{Name: "__scheme__", Value: "http"},
				labels.Label{Name: "__meta_kubernetes_pod_name", Value: "my-pod-23491"},
				labels.Label{Name: "__meta_kubernetes_pod_uid", Value: "84279wretgu89dg489q2"},
				labels.Label{Name: "__meta_kubernetes_pod_container_name", Value: "my-container"},
				labels.Label{Name: "__meta_kubernetes_pod_node_name", Value: "k8s-node-123"},
				labels.Label{Name: "__meta_kubernetes_pod_controller_name", Value: "my-pod"},
				labels.Label{Name: "__meta_kubernetes_pod_controller_kind", Value: "Job"},
				labels.Label{Name: "__meta_kubernetes_namespace", Value: "kube-system"},
			),
			removeOldSemconvFeatureGate: true,
			want: makeK8sResource(&jobInstanceDefinition{
				"job", "hostname:8888", "hostname", "http", "8888",
			}, &k8sResourceDefinition{
				podName:   "my-pod-23491",
				podUID:    "84279wretgu89dg489q2",
				container: "my-container",
				node:      "k8s-node-123",
				job:       "my-pod",
				ns:        "kube-system",
			}),
		},
		{
			name: "kubernetes cronjob pod",
			job:  "job", instance: "hostname:8888", sdLabels: labels.New(
				labels.Label{Name: "__scheme__", Value: "http"},
				labels.Label{Name: "__meta_kubernetes_pod_name", Value: "my-pod-23491"},
				labels.Label{Name: "__meta_kubernetes_pod_uid", Value: "84279wretgu89dg489q2"},
				labels.Label{Name: "__meta_kubernetes_pod_container_name", Value: "my-container"},
				labels.Label{Name: "__meta_kubernetes_pod_node_name", Value: "k8s-node-123"},
				labels.Label{Name: "__meta_kubernetes_pod_controller_name", Value: "my-pod"},
				labels.Label{Name: "__meta_kubernetes_pod_controller_kind", Value: "CronJob"},
				labels.Label{Name: "__meta_kubernetes_namespace", Value: "kube-system"},
			),
			removeOldSemconvFeatureGate: true,
			want: makeK8sResource(&jobInstanceDefinition{
				"job", "hostname:8888", "hostname", "http", "8888",
			}, &k8sResourceDefinition{
				podName:   "my-pod-23491",
				podUID:    "84279wretgu89dg489q2",
				container: "my-container",
				node:      "k8s-node-123",
				cronjob:   "my-pod",
				ns:        "kube-system",
			}),
		},
		{
			name: "kubernetes node (e.g. kubelet)",
			job:  "job", instance: "hostname:8888", sdLabels: labels.New(
				labels.Label{Name: "__scheme__", Value: "http"},
				labels.Label{Name: "__meta_kubernetes_node_name", Value: "k8s-node-123"},
			),
			removeOldSemconvFeatureGate: true,
			want: makeK8sResource(&jobInstanceDefinition{
				"job", "hostname:8888", "hostname", "http", "8888",
			}, &k8sResourceDefinition{
				node: "k8s-node-123",
			}),
		},
		{
			name: "kubernetes service endpoint",
			job:  "job", instance: "hostname:8888", sdLabels: labels.New(
				labels.Label{Name: "__scheme__", Value: "http"},
				labels.Label{Name: "__meta_kubernetes_endpoint_node_name", Value: "k8s-node-123"},
			),
			removeOldSemconvFeatureGate: true,
			want: makeK8sResource(&jobInstanceDefinition{
				"job", "hostname:8888", "hostname", "http", "8888",
			}, &k8sResourceDefinition{
				node: "k8s-node-123",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.SetFeatureGateForTest(t, removeOldSemconvFeatureGate, tt.removeOldSemconvFeatureGate)
			got := CreateResource(tt.job, tt.instance, tt.sdLabels)
			require.Equal(t, tt.want.Attributes().AsRaw(), got.Attributes().AsRaw())
		})
	}
}
