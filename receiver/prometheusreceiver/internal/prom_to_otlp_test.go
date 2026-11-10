// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
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
		attrs.PutStr("k8s.pod.name", def.podName)
	}
	if def.podUID != "" {
		attrs.PutStr("k8s.pod.uid", def.podUID)
	}
	if def.container != "" {
		attrs.PutStr("k8s.container.name", def.container)
	}
	if def.node != "" {
		attrs.PutStr("k8s.node.name", def.node)
	}
	if def.rs != "" {
		attrs.PutStr("k8s.replicaset.name", def.rs)
	}
	if def.ds != "" {
		attrs.PutStr("k8s.daemonset.name", def.ds)
	}
	if def.ss != "" {
		attrs.PutStr("k8s.statefulset.name", def.ss)
	}
	if def.job != "" {
		attrs.PutStr("k8s.job.name", def.job)
	}
	if def.cronjob != "" {
		attrs.PutStr("k8s.cronjob.name", def.cronjob)
	}
	if def.ns != "" {
		attrs.PutStr("k8s.namespace.name", def.ns)
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

func TestCreateNodeAndResourcePromToOTLP(t *testing.T) {
	tests := []struct {
		name, job string
		instance  string
		sdLabels  labels.Labels
		want      pcommon.Resource
	}{
		{
			name: "all attributes proper",
			job:  "job", instance: "hostname:8888", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
				"job", "hostname:8888", "hostname", "http", "8888",
			}, true),
		},
		{
			name: "missing port",
			job:  "job", instance: "myinstance", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "https"}),
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
				"job", "myinstance", "myinstance", "https", "",
			}, true),
		},
		{
			name: "blank scheme",
			job:  "job", instance: "myinstance:443", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: ""}),
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
				"job", "myinstance:443", "myinstance", "", "443",
			}, true),
		},
		{
			name: "blank instance, blank scheme",
			job:  "job", instance: "", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: ""}),
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
				"job", "", "", "", "",
			}, true),
		},
		{
			name: "blank instance, non-blank scheme",
			job:  "job", instance: "", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
				"job", "", "", "http", "",
			}, true),
		},
		{
			name: "0.0.0.0 address",
			job:  "job", instance: "0.0.0.0:8888", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
				"job", "0.0.0.0:8888", "", "http", "8888",
			}, false),
		},
		{
			name: "localhost",
			job:  "job", instance: "localhost:8888", sdLabels: labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: makeResourceWithJobInstanceScheme(&jobInstanceDefinition{
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
			want: makeK8sResource(&jobInstanceDefinition{
				"job", "hostname:8888", "hostname", "http", "8888",
			}, &k8sResourceDefinition{
				node: "k8s-node-123",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateResource(tt.job, tt.instance, "", "", tt.sdLabels)
			require.Equal(t, tt.want.Attributes().AsRaw(), got.Attributes().AsRaw())
		})
	}
}

func TestCreateResourceWithTargetURL(t *testing.T) {
	err := featuregate.GlobalRegistry().Set(metadata.ReceiverPrometheusUseTargetURLForServerAddressFeatureGate.ID(), true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverPrometheusUseTargetURLForServerAddressFeatureGate.ID(), false))
	})

	tests := []struct {
		name          string
		job           string
		instance      string
		targetAddress string
		targetScheme  string
		sdLabels      labels.Labels
		want          map[string]any
	}{
		{
			name:          "Host with port",
			job:           "job",
			instance:      "example.com:8080",
			targetAddress: "example.com:8080",
			targetScheme:  "http",
			sdLabels:      labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: map[string]any{
				"service.name":        "job",
				"service.instance.id": "example.com:8080",
				"server.address":      "example.com",
				"server.port":         "8080",
				"url.scheme":          "http",
			},
		},
		{
			name:          "Relabeled non-host:port instance",
			job:           "job",
			instance:      "logical-instance-1",
			targetAddress: "example.com:8080",
			targetScheme:  "http",
			sdLabels:      labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: map[string]any{
				"service.name":        "job",
				"service.instance.id": "logical-instance-1",
				"server.address":      "example.com",
				"server.port":         "8080",
				"url.scheme":          "http",
			},
		},
		{
			name:          "Relabeled targetScheme overrides sdLabels __scheme__",
			job:           "job",
			instance:      "example.com:8443",
			targetAddress: "example.com:8443",
			targetScheme:  "https",
			sdLabels:      labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: map[string]any{
				"service.name":        "job",
				"service.instance.id": "example.com:8443",
				"server.address":      "example.com",
				"server.port":         "8443",
				"url.scheme":          "https",
			},
		},
		{
			name:          "Dropped targetScheme does not fall back to sdLabels",
			job:           "job",
			instance:      "example.com:8080",
			targetAddress: "example.com:8080",
			targetScheme:  "",
			sdLabels:      labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: map[string]any{
				"service.name":        "job",
				"service.instance.id": "example.com:8080",
				"server.address":      "example.com",
				"server.port":         "8080",
			},
		},
		{
			name:          "IPv6 with port",
			job:           "job",
			instance:      "[2001:db8::1]:9090",
			targetAddress: "[2001:db8::1]:9090",
			targetScheme:  "http",
			sdLabels:      labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: map[string]any{
				"service.name":        "job",
				"service.instance.id": "[2001:db8::1]:9090",
				"server.address":      "2001:db8::1",
				"server.port":         "9090",
				"url.scheme":          "http",
			},
		},
		{
			name:          "Portless hostname",
			job:           "job",
			instance:      "example.com",
			targetAddress: "example.com",
			targetScheme:  "http",
			sdLabels:      labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: map[string]any{
				"service.name":        "job",
				"service.instance.id": "example.com",
				"server.address":      "example.com",
				"url.scheme":          "http",
			},
		},
		{
			name:          "Portless IPv6",
			job:           "job",
			instance:      "[2001:db8::1]",
			targetAddress: "[2001:db8::1]",
			targetScheme:  "http",
			sdLabels:      labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: map[string]any{
				"service.name":        "job",
				"service.instance.id": "[2001:db8::1]",
				"server.address":      "2001:db8::1",
				"url.scheme":          "http",
			},
		},
		{
			name:          "Loopback IPv6 without port [::1]",
			job:           "job",
			instance:      "logical-local",
			targetAddress: "[::1]",
			targetScheme:  "http",
			sdLabels:      labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: map[string]any{
				"service.name":        "job",
				"service.instance.id": "logical-local",
				"url.scheme":          "http",
			},
		},
		{
			name:          "Loopback IPv6 with port [::1]:9090",
			job:           "job",
			instance:      "[::1]:9090",
			targetAddress: "[::1]:9090",
			targetScheme:  "http",
			sdLabels:      labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: map[string]any{
				"service.name":        "job",
				"service.instance.id": "[::1]:9090",
				"server.port":         "9090",
				"url.scheme":          "http",
			},
		},
		{
			name:          "Loopback IPv4 with port 127.0.0.1:9090",
			job:           "job",
			instance:      "127.0.0.1:9090",
			targetAddress: "127.0.0.1:9090",
			targetScheme:  "http",
			sdLabels:      labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: map[string]any{
				"service.name":        "job",
				"service.instance.id": "127.0.0.1:9090",
				"server.port":         "9090",
				"url.scheme":          "http",
			},
		},
		{
			name:          "Unspecified IPv6 with port [::]:9090",
			job:           "job",
			instance:      "[::]:9090",
			targetAddress: "[::]:9090",
			targetScheme:  "http",
			sdLabels:      labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: map[string]any{
				"service.name":        "job",
				"service.instance.id": "[::]:9090",
				"server.port":         "9090",
				"url.scheme":          "http",
			},
		},
		{
			name:          "Unix socket / empty targetAddress omits server.address and server.port",
			job:           "job",
			instance:      "unix-target:8080",
			targetAddress: "",
			targetScheme:  "http",
			sdLabels:      labels.New(labels.Label{Name: "__scheme__", Value: "http"}),
			want: map[string]any{
				"service.name":        "job",
				"service.instance.id": "unix-target:8080",
				"url.scheme":          "http",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateResource(tt.job, tt.instance, tt.targetAddress, tt.targetScheme, tt.sdLabels)
			require.Equal(t, tt.want, got.Attributes().AsRaw())
		})
	}
}
