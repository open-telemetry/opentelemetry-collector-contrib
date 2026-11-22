// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package receivercreator

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

const (
	testKubeConfig      = "/tmp/kube-config-otelcol-e2e-testing"
	e2eWorkloadsDir     = "workloads"
	e2eCollectorDir     = "collector"
	e2eWorkloadPrefix   = "receivercreator-e2e-"
	parsedByTemplate    = "fluentbit-annotation"
	parsedByDiscovery   = "discovery-annotation"
	minRecordsPerPod    = 3
	duplicateCheckDelay = 5 * time.Second
	linesAfterChange    = 5
)

// podLogs summarizes the log records received for one pod.
type podLogs struct {
	// records counts the records per value of the parsed_by attribute.
	records map[string]int
	// seqCounts counts how many times each seq value was received.
	seqCounts map[string]int
	// unparsed counts the records without the attributes set by the parser.
	unparsed int
}

func (p *podLogs) duplicates() int {
	duplicates := 0
	for _, count := range p.seqCounts {
		if count > 1 {
			duplicates++
		}
	}
	return duplicates
}

func collectPodLogs(sink *consumertest.LogsSink) map[string]*podLogs {
	pods := map[string]*podLogs{}
	for _, logs := range sink.AllLogs() {
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			rl := logs.ResourceLogs().At(i)
			podName, ok := rl.Resource().Attributes().Get("k8s.pod.name")
			if !ok {
				continue
			}
			pod, ok := pods[podName.Str()]
			if !ok {
				pod = &podLogs{records: map[string]int{}, seqCounts: map[string]int{}}
				pods[podName.Str()] = pod
			}
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				records := rl.ScopeLogs().At(j).LogRecords()
				for k := 0; k < records.Len(); k++ {
					attrs := records.At(k).Attributes()
					parsedBy, hasParsedBy := attrs.Get("parsed_by")
					seq, hasSeq := attrs.Get("seq")
					level, hasLevel := attrs.Get("level")
					if !hasParsedBy || !hasSeq || !hasLevel || level.Str() != "INFO" {
						pod.unparsed++
						continue
					}
					pod.records[parsedBy.Str()]++
					pod.seqCounts[seq.Str()]++
				}
			}
		}
	}
	return pods
}

// TestE2ELogsFromTemplatesAndDiscoveryAnnotations runs the receiver creator with a file log
// template selected by a Fluent Bit annotation next to annotation-based discovery, the setup
// used while workloads move from one set of annotations to the other.
func TestE2ELogsFromTemplatesAndDiscoveryAnnotations(t *testing.T) {
	_, logsConsumer := startWorkloadsAndCollector(t)

	// parsed_by value expected for each workload.
	expectedParsedBy := map[string]string{
		// Only the Fluent Bit annotation: the template is used.
		e2eWorkloadPrefix + "fluentbit-annotation": parsedByTemplate,
		// Only the discovery annotations: the receiver comes from the annotations.
		e2eWorkloadPrefix + "discovery-annotation": parsedByDiscovery,
		// Both sets of annotations: discovery wins and the template is not used.
		e2eWorkloadPrefix + "both-annotations": parsedByDiscovery,
		// Invalid metrics discovery annotations on the pod's ports must not stop the logs
		// of this pod or of the other pods from being collected.
		e2eWorkloadPrefix + "invalid-annotation": parsedByTemplate,
	}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pods := collectPodLogs(logsConsumer)
		for podName, parsedBy := range expectedParsedBy {
			pod, ok := pods[podName]
			if !assert.Truef(c, ok, "no logs received for pod %s", podName) {
				continue
			}
			assert.GreaterOrEqualf(c, pod.records[parsedBy], minRecordsPerPod, "pod %s", podName)
		}
	}, 3*time.Minute, time.Second, "did not receive parsed logs for every workload")

	// Give a second receiver for the same container the time to show up.
	time.Sleep(duplicateCheckDelay)

	pods := collectPodLogs(logsConsumer)
	for podName, parsedBy := range expectedParsedBy {
		pod := pods[podName]
		require.NotNilf(t, pod, "pod %s", podName)
		assert.Lenf(t, pod.records, 1, "pod %s was read by more than one receiver: %v", podName, pod.records)
		assert.Positivef(t, pod.records[parsedBy], "pod %s", podName)
		assert.Zerof(t, pod.unparsed, "pod %s has records the parser did not handle", podName)
		assert.Zerof(t, pod.duplicates(), "pod %s has lines received more than once", podName)
	}
}

// TestE2EReceiverKeptWhenPodChanges changes the labels of a running pod. The config of the
// file log receiver of the pod is the same after the change, so the receiver keeps running
// and does not read the log file again from the start.
func TestE2EReceiverKeptWhenPodChanges(t *testing.T) {
	k8sClient, logsConsumer := startWorkloadsAndCollector(t)

	podName := e2eWorkloadPrefix + "fluentbit-annotation"
	received := func() int {
		pod, ok := collectPodLogs(logsConsumer)[podName]
		if !ok {
			return 0
		}
		return pod.records[parsedByTemplate]
	}
	require.Eventually(t, func() bool {
		return received() >= minRecordsPerPod
	}, 3*time.Minute, time.Second, "did not receive logs before the change")

	_, err := podsClient(k8sClient).Patch(t.Context(), podName, types.MergePatchType,
		[]byte(`{"metadata":{"labels":{"receivercreator-e2e-changed":"true"}}}`), metav1.PatchOptions{})
	require.NoError(t, err, "failed changing the labels of the pod")

	// Lines written after the change arrive, so the change was handled and the pod is still read.
	afterChange := received() + linesAfterChange
	require.Eventually(t, func() bool {
		return received() >= afterChange
	}, time.Minute, time.Second, "did not receive logs after the change")

	// Give a receiver that was started again the time to deliver what it read again.
	time.Sleep(duplicateCheckDelay)

	pod := collectPodLogs(logsConsumer)[podName]
	assert.Zero(t, pod.duplicates(), "lines were received again after the change")
	assert.Zero(t, pod.unparsed)
}

// startWorkloadsAndCollector starts a receiver for what the collector exports, the workloads and
// then the collector. The workloads run before the collector starts, so the observer reports
// all of their endpoints to the receiver creator in one batch.
func startWorkloadsAndCollector(t *testing.T) (*k8stest.K8sClient, *consumertest.LogsSink) {
	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	getOrInsertDefault(t, &cfg.Protocols.GRPC).NetAddr.Endpoint = "0.0.0.0:4317"
	logsConsumer := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(f.Type()), cfg, logsConsumer)
	require.NoError(t, err, "failed creating logs receiver")
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		assert.NoError(t, rcvr.Shutdown(context.WithoutCancel(t.Context())))
	})

	deleteObjects := func(objs []*unstructured.Unstructured) {
		for _, obj := range objs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}

	workloadObjs, err := k8stest.CreateObjects(k8sClient, filepath.Join("testdata", "e2e", e2eWorkloadsDir))
	require.NoError(t, err, "failed creating workloads")
	t.Cleanup(func() {
		deleteObjects(workloadObjs)
		// The next test creates the same pods, so wait until these are gone.
		require.Eventually(t, func() bool {
			pods, listErr := podsClient(k8sClient).List(context.WithoutCancel(t.Context()), metav1.ListOptions{LabelSelector: "app=receivercreator-e2e-workload"})
			return listErr == nil && len(pods.Items) == 0
		}, time.Minute, time.Second, "the workloads were not deleted")
	})
	k8stest.WaitForCollectorToStart(t, k8sClient, "default", map[string]any{"app": "receivercreator-e2e-workload"})

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join("testdata", "e2e", e2eCollectorDir), map[string]string{}, "")
	t.Cleanup(func() { deleteObjects(collectorObjs) })

	return k8sClient, logsConsumer
}

func podsClient(k8sClient *k8stest.K8sClient) dynamic.ResourceInterface {
	return k8sClient.DynamicClient.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).Namespace("default")
}

// getOrInsertDefault is a helper function to get or insert a default value for a configoptional.Optional type.
func getOrInsertDefault[T any](t *testing.T, opt *configoptional.Optional[T]) *T {
	if opt.HasValue() {
		return opt.Get()
	}

	empty := confmap.NewFromStringMap(map[string]any{})
	require.NoError(t, empty.Unmarshal(opt))
	val := opt.Get()
	require.NotNil(t, "Expected a default value to be set for %T", val)
	return val
}
