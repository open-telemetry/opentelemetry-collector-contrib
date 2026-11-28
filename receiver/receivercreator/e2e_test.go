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
	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	getOrInsertDefault(t, &cfg.Protocols.GRPC).NetAddr.Endpoint = "0.0.0.0:4317"
	logsConsumer := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, logsConsumer)
	require.NoError(t, err, "failed creating logs receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	// The workloads run before the collector starts, so the observer reports all of their
	// endpoints to the receiver creator in one batch.
	workloadObjs, err := k8stest.CreateObjects(k8sClient, filepath.Join("testdata", "e2e", e2eWorkloadsDir))
	require.NoError(t, err, "failed creating workloads")
	defer func() {
		for _, obj := range workloadObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()
	k8stest.WaitForCollectorToStart(t, k8sClient, "default", map[string]any{"app": "receivercreator-e2e-workload"})

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join("testdata", "e2e", e2eCollectorDir), map[string]string{}, "")
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

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
