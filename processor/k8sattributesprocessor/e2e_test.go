// Copyright The OpenTelemetry Authors
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

//go:build e2e
// +build e2e

package k8sattributesprocessor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/multierr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	equal = iota
	regex
	exist
)

const testKubeConfig = "/tmp/kube-config-otelcol-e2e-testing"

type expectedValue struct {
	mode  int
	value string
}

func newExpectedValue(mode int, value string) *expectedValue {
	return &expectedValue{
		mode:  mode,
		value: value,
	}
}

// TestE2E tests the k8s attributes processor with a real k8s cluster.
// The test requires a prebuilt otelcontribcol image uploaded to a kind k8s cluster defined in
// `/tmp/kube-config-otelcol-e2e-testing`. Run the following command prior to running the test locally:
//
//	kind create cluster --kubeconfig=/tmp/kube-config-otelcol-e2e-testing
//	make docker-otelcontribcol
//	KUBECONFIG=/tmp/kube-config-otelcol-e2e-testing kind load docker-image otelcontribcol:latest
func TestE2E(t *testing.T) {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	require.NoError(t, err)

	testID := uuid.NewString()[:8]
	collectorObjs := createCollectorObjects(t, dynamicClient, testID)
	telemetryGenObjs := createTelemetryGenObjects(t, dynamicClient, testID)
	defer func() {
		for _, obj := range append(collectorObjs, telemetryGenObjs...) {
			require.NoErrorf(t, deleteObject(dynamicClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	metricsConsumer := new(consumertest.MetricsSink)
	tracesConsumer := new(consumertest.TracesSink)
	logsConsumer := new(consumertest.LogsSink)
	wantEntries := 128 // Minimal number of metrics/traces/logs to wait for.
	waitForData(t, wantEntries, metricsConsumer, tracesConsumer, logsConsumer)

	tcs := []struct {
		name     string
		dataType component.DataType
		service  string
		attrs    map[string]*expectedValue
	}{
		{
			name:     "traces-job",
			dataType: component.DataTypeTraces,
			service:  "test-traces-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-traces-job-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.job.name":             newExpectedValue(equal, "telemetrygen-"+testID+"-traces-job"),
				"k8s.job.uid":              newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "job"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-traces-job"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "traces-statefulset",
			dataType: component.DataTypeTraces,
			service:  "test-traces-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset-0"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.statefulset.name":     newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset"),
				"k8s.statefulset.uid":      newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "traces-deployment",
			dataType: component.DataTypeTraces,
			service:  "test-traces-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.deployment.name":      newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.replicaset.name":      newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":       newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "deployment"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "traces-daemonset",
			dataType: component.DataTypeTraces,
			service:  "test-traces-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-traces-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.daemonset.name":       newExpectedValue(equal, "telemetrygen-"+testID+"-traces-daemonset"),
				"k8s.daemonset.uid":        newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-traces-daemonset"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "metrics-job",
			dataType: component.DataTypeMetrics,
			service:  "test-metrics-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-job-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.job.name":             newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-job"),
				"k8s.job.uid":              newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "job"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-job"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "metrics-statefulset",
			dataType: component.DataTypeMetrics,
			service:  "test-metrics-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-statefulset-0"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.statefulset.name":     newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-statefulset"),
				"k8s.statefulset.uid":      newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-statefulset"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "metrics-deployment",
			dataType: component.DataTypeMetrics,
			service:  "test-metrics-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.deployment.name":      newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.replicaset.name":      newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":       newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "deployment"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "metrics-daemonset",
			dataType: component.DataTypeMetrics,
			service:  "test-metrics-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.daemonset.name":       newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-daemonset"),
				"k8s.daemonset.uid":        newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-daemonset"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "logs-job",
			dataType: component.DataTypeLogs,
			service:  "test-logs-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-logs-job-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.job.name":             newExpectedValue(equal, "telemetrygen-"+testID+"-logs-job"),
				"k8s.job.uid":              newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "job"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-logs-job"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "logs-statefulset",
			dataType: component.DataTypeLogs,
			service:  "test-logs-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(equal, "telemetrygen-"+testID+"-logs-statefulset-0"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.statefulset.name":     newExpectedValue(equal, "telemetrygen-"+testID+"-logs-statefulset"),
				"k8s.statefulset.uid":      newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-logs-statefulset"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "logs-deployment",
			dataType: component.DataTypeLogs,
			service:  "test-logs-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.deployment.name":      newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.replicaset.name":      newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":       newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "deployment"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "logs-daemonset",
			dataType: component.DataTypeLogs,
			service:  "test-logs-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-logs-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.daemonset.name":       newExpectedValue(equal, "telemetrygen-"+testID+"-logs-daemonset"),
				"k8s.daemonset.uid":        newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-logs-daemonset"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.dataType {
			case component.DataTypeTraces:
				scanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs)
			case component.DataTypeMetrics:
				scanMetricsForAttributes(t, metricsConsumer, tc.service, tc.attrs)
			case component.DataTypeLogs:
				scanLogsForAttributes(t, logsConsumer, tc.service, tc.attrs)
			default:
				t.Fatalf("unknown data type %s", tc.dataType)
			}
		})
	}
}

func scanTracesForAttributes(t *testing.T, ts *consumertest.TracesSink, expectedService string,
	kvs map[string]*expectedValue) {
	// Iterate over the received set of traces starting from the most recent entries due to a bug in the processor:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18892
	// TODO: Remove the reverse loop once it's fixed. All the metrics should be properly annotated.
	for i := len(ts.AllTraces()) - 1; i >= 0; i-- {
		traces := ts.AllTraces()[i]
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			resource := traces.ResourceSpans().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "span do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, resourceHasAttributes(resource, kvs))
			return
		}
	}
	t.Fatalf("no spans found for service %s", expectedService)
}

func scanMetricsForAttributes(t *testing.T, ms *consumertest.MetricsSink, expectedService string,
	kvs map[string]*expectedValue) {
	// Iterate over the received set of metrics starting from the most recent entries due to a bug in the processor:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18892
	// TODO: Remove the reverse loop once it's fixed. All the metrics should be properly annotated.
	for i := len(ms.AllMetrics()) - 1; i >= 0; i-- {
		metrics := ms.AllMetrics()[i]
		for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
			resource := metrics.ResourceMetrics().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "metric do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, resourceHasAttributes(resource, kvs))
			return
		}
	}
	t.Fatalf("no metric found for service %s", expectedService)
}

func scanLogsForAttributes(t *testing.T, ls *consumertest.LogsSink, expectedService string,
	kvs map[string]*expectedValue) {
	// Iterate over the received set of logs starting from the most recent entries due to a bug in the processor:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18892
	// TODO: Remove the reverse loop once it's fixed. All the metrics should be properly annotated.
	for i := len(ls.AllLogs()) - 1; i >= 0; i-- {
		logs := ls.AllLogs()[i]
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			resource := logs.ResourceLogs().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "log do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, resourceHasAttributes(resource, kvs))
			return
		}
	}
	t.Fatalf("no logs found for service %s", expectedService)
}

func resourceHasAttributes(resource pcommon.Resource, kvs map[string]*expectedValue) error {
	foundAttrs := make(map[string]bool)
	for k := range kvs {
		foundAttrs[k] = false
	}

	resource.Attributes().Range(
		func(k string, v pcommon.Value) bool {
			if val, ok := kvs[k]; ok {
				switch val.mode {
				case equal:
					if val.value == v.AsString() {
						foundAttrs[k] = true
					}
				case regex:
					matched, _ := regexp.MatchString(val.value, v.AsString())
					if matched {
						foundAttrs[k] = true
					}
				case exist:
					foundAttrs[k] = true
				}

			}
			return true
		},
	)

	var err error
	for k, v := range foundAttrs {
		if !v {
			err = multierr.Append(err, fmt.Errorf("%v attribute not found", k))
		}
	}
	return err
}

func createCollectorObjects(t *testing.T, client *dynamic.DynamicClient, testID string) []*unstructured.Unstructured {
	manifestsDir := filepath.Join(".", "testdata", "e2e", "collector")
	manifestFiles, err := os.ReadDir(manifestsDir)
	require.NoErrorf(t, err, "failed to read collector manifests directory %s", manifestsDir)
	host := hostEndpoint(t)
	var podNamespace string
	var podLabels map[string]any
	createdObjs := make([]*unstructured.Unstructured, 0, len(manifestFiles))
	for _, manifestFile := range manifestFiles {
		tmpl := template.Must(template.New(manifestFile.Name()).ParseFiles(filepath.Join(manifestsDir, manifestFile.Name())))
		manifest := &bytes.Buffer{}
		require.NoError(t, tmpl.Execute(manifest, map[string]string{
			"Name":         "otelcol-" + testID,
			"HostEndpoint": host,
		}))
		obj, err := createObject(client, manifest.Bytes())
		require.NoErrorf(t, err, "failed to create collector object from manifest %s", manifestFile.Name())
		if obj.GetKind() == "Deployment" {
			podNamespace = obj.GetNamespace()
			selector := obj.Object["spec"].(map[string]any)["selector"]
			podLabels = selector.(map[string]any)["matchLabels"].(map[string]any)
		}
		createdObjs = append(createdObjs, obj)
	}

	waitForCollectorToStart(t, client, podNamespace, podLabels)

	return createdObjs
}

func waitForCollectorToStart(t *testing.T, client *dynamic.DynamicClient, podNamespace string, podLabels map[string]any) {
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	listOptions := metav1.ListOptions{LabelSelector: selectorFromMap(podLabels).String()}
	podTimeoutMinutes := 3
	var podPhase string
	require.Eventually(t, func() bool {
		list, err := client.Resource(podGVR).Namespace(podNamespace).List(context.Background(), listOptions)
		require.NoError(t, err, "failed to list collector pods")
		if len(list.Items) == 0 {
			return false
		}
		podPhase = list.Items[0].Object["status"].(map[string]interface{})["phase"].(string)
		return podPhase == "Running"
	}, time.Duration(podTimeoutMinutes)*time.Minute, 50*time.Millisecond,
		"collector pod haven't started within %d minutes, latest pod phase is %s", podTimeoutMinutes, podPhase)
}

func selectorFromMap(labelMap map[string]any) labels.Selector {
	labelStringMap := make(map[string]string)
	for key, value := range labelMap {
		labelStringMap[key] = value.(string)
	}
	labelSet := labels.Set(labelStringMap)
	return labelSet.AsSelector()
}

func createTelemetryGenObjects(t *testing.T, client *dynamic.DynamicClient, testID string) []*unstructured.Unstructured {
	manifestsDir := filepath.Join(".", "testdata", "e2e", "telemetrygen")
	manifestFiles, err := os.ReadDir(manifestsDir)
	require.NoErrorf(t, err, "failed to read telemetrygen manifests directory %s", manifestsDir)
	createdObjs := make([]*unstructured.Unstructured, 0, len(manifestFiles))
	for _, manifestFile := range manifestFiles {
		tmpl := template.Must(template.New(manifestFile.Name()).ParseFiles(filepath.Join(manifestsDir, manifestFile.Name())))
		for _, dataType := range []string{"metrics", "logs", "traces"} {
			manifest := &bytes.Buffer{}
			require.NoError(t, tmpl.Execute(manifest, map[string]string{
				"Name":         "telemetrygen-" + testID,
				"DataType":     dataType,
				"OTLPEndpoint": "otelcol-" + testID + ":4317",
			}))
			obj, err := createObject(client, manifest.Bytes())
			require.NoErrorf(t, err, "failed to create telemetrygen object from manifest %s", manifestFile.Name())
			createdObjs = append(createdObjs, obj)
		}
	}
	return createdObjs
}

func createObject(client *dynamic.DynamicClient, manifest []byte) (*unstructured.Unstructured, error) {
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, gvk, err := decoder.Decode(manifest, nil, obj)
	if err != nil {
		return nil, err
	}
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: strings.ToLower(gvk.Kind + "s"),
	}
	return client.Resource(gvr).Namespace(obj.GetNamespace()).Create(context.Background(), obj, metav1.CreateOptions{})
}

func deleteObject(client *dynamic.DynamicClient, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: strings.ToLower(gvk.Kind + "s"),
	}
	return client.Resource(gvr).Namespace(obj.GetNamespace()).Delete(context.Background(), obj.GetName(), metav1.DeleteOptions{})
}

func waitForData(t *testing.T, entriesNum int, mc *consumertest.MetricsSink, tc *consumertest.TracesSink, lc *consumertest.LogsSink) {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)

	_, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mc)
	require.NoError(t, err, "failed creating metrics receiver")
	_, err = f.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, tc)
	require.NoError(t, err, "failed creating traces receiver")
	rcvr, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, lc)
	require.NoError(t, err, "failed creating logs receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) > entriesNum && len(tc.AllTraces()) > entriesNum && len(lc.AllLogs()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d metrics, %d traces, %d logs in %d minutes", entriesNum,
		len(mc.AllMetrics()), len(tc.AllTraces()), len(lc.AllLogs()), timeoutMinutes)
}

func hostEndpoint(t *testing.T) string {
	if runtime.GOOS == "darwin" {
		return "host.docker.internal"
	}

	client, err := docker.NewClientWithOpts(docker.FromEnv)
	require.NoError(t, err)
	client.NegotiateAPIVersion(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	network, err := client.NetworkInspect(ctx, "kind", types.NetworkInspectOptions{})
	require.NoError(t, err)
	for _, ipam := range network.IPAM.Config {
		return ipam.Gateway
	}
	require.Fail(t, "failed to find host endpoint")
	return ""
}
