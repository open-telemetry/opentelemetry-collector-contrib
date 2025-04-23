// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package k8sclusterreceiver

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

const (
	expectedFileClusterScoped   = "./testdata/e2e/cluster-scoped/expected.yaml"
	expectedFileNamespaceScoped = "./testdata/e2e/namespace-scoped/expected.yaml"

	testObjectsDirClusterScoped   = "./testdata/e2e/cluster-scoped/testobjects"
	testObjectsDirNamespaceScoped = "./testdata/e2e/namespace-scoped/testobjects"
	testKubeConfig                = "/tmp/kube-config-otelcol-e2e-testing"
)

// TestE2EClusterScoped tests the k8s cluster receiver with a real k8s cluster.
// The test requires a prebuilt otelcontribcol image uploaded to a kind k8s cluster defined in
// `/tmp/kube-config-otelcol-e2e-testing`. Run the following command prior to running the test locally:
//
//	kind create cluster --kubeconfig=/tmp/kube-config-otelcol-e2e-testing
//	make docker-otelcontribcol
//	KUBECONFIG=/tmp/kube-config-otelcol-e2e-testing kind load docker-image otelcontribcol:latest
func TestE2EClusterScoped(t *testing.T) {
	var expected pmetric.Metrics
	expected, err := golden.ReadMetrics(expectedFileClusterScoped)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	// k8s test objs
	testObjs, err := k8stest.CreateObjects(k8sClient, testObjectsDirClusterScoped)
	require.NoErrorf(t, err, "failed to create objects")

	t.Cleanup(func() {
		require.NoErrorf(t, k8stest.DeleteObjects(k8sClient, testObjs), "failed to delete objects")
	})

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "cluster-scoped", "collector"), map[string]string{}, "")

	t.Cleanup(func() {
		for _, obj := range append(collectorObjs) {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	})

	// CronJob is scheduled to be executed every minute (on the full minute)
	// This creates a delay and the resources deployed by CronJob (Job, Pod, Container)
	// might be available later and won't make it to the resulting metrics, which may cause the test to fail
	time.Sleep(calculateCronJobExecution())

	wantEntries := 10 // Minimal number of metrics to wait for.
	// the commented line below writes the received list of metrics to the expected.yaml
	// golden.WriteMetrics(t, expectedFileClusterScoped, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])
	waitForData(t, wantEntries, metricsConsumer)

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreMetricValues(
				"k8s.container.cpu_request",
				"k8s.container.memory_limit",
				"k8s.container.memory_request",
				"k8s.container.restarts",
				"k8s.cronjob.active_jobs",
				"k8s.deployment.available",
				"k8s.deployment.desired",
				"k8s.job.active_pods",
				"k8s.job.desired_successful_pods",
				"k8s.job.failed_pods",
				"k8s.job.max_parallel_pods",
				"k8s.hpa.current_replicas",
				"k8s.job.successful_pods"),
			pmetrictest.ChangeResourceAttributeValue("container.id", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("container.image.name", containerImageShorten),
			pmetrictest.ChangeResourceAttributeValue("container.image.tag", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.cronjob.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.daemonset.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.deployment.name", shortenNames),
			pmetrictest.ChangeResourceAttributeValue("k8s.deployment.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.hpa.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.job.name", shortenNames),
			pmetrictest.ChangeResourceAttributeValue("k8s.job.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.namespace.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.node.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.pod.name", shortenNames),
			pmetrictest.ChangeResourceAttributeValue("k8s.pod.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.name", shortenNames),
			pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.statefulset.uid", replaceWithStar),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2ENamespaceScoped tests the k8s cluster receiver with a real k8s cluster.
// The test requires a prebuilt otelcontribcol image uploaded to a kind k8s cluster defined in
// `/tmp/kube-config-otelcol-e2e-testing`. Run the following command prior to running the test locally:
//
//	kind create cluster --kubeconfig=/tmp/kube-config-otelcol-e2e-testing
//	make docker-otelcontribcol
//	KUBECONFIG=/tmp/kube-config-otelcol-e2e-testing kind load docker-image otelcontribcol:latest
func TestE2ENamespaceScoped(t *testing.T) {
	var expected pmetric.Metrics
	expected, err := golden.ReadMetrics(expectedFileNamespaceScoped)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	// k8s test objs
	testObjs, err := k8stest.CreateObjects(k8sClient, testObjectsDirNamespaceScoped)
	require.NoErrorf(t, err, "failed to create objects")

	t.Cleanup(func() {
		require.NoErrorf(t, k8stest.DeleteObjects(k8sClient, testObjs), "failed to delete objects")
	})

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "namespace-scoped", "collector"), map[string]string{}, "")

	t.Cleanup(func() {
		for _, obj := range append(collectorObjs) {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	})

	// CronJob is scheduled to be executed every minute (on the full minute)
	// This creates a delay and the resources deployed by CronJob (Job, Pod, Container)
	// might be available later and won't make it to the resulting metrics, which may cause the test to fail
	time.Sleep(calculateCronJobExecution())

	wantEntries := 10 // Minimal number of metrics to wait for.
	// the commented line below writes the received list of metrics to the expected.yaml
	// golden.WriteMetrics(t, expectedFileNamespaceScoped, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])
	waitForData(t, wantEntries, metricsConsumer)

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreMetricValues(
				"k8s.container.cpu_request",
				"k8s.container.memory_limit",
				"k8s.container.memory_request",
				"k8s.container.restarts",
				"k8s.cronjob.active_jobs",
				"k8s.deployment.available",
				"k8s.deployment.desired",
				"k8s.job.active_pods",
				"k8s.job.desired_successful_pods",
				"k8s.job.failed_pods",
				"k8s.job.max_parallel_pods",
				"k8s.hpa.current_replicas",
				"k8s.job.successful_pods"),
			pmetrictest.ChangeResourceAttributeValue("container.id", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("container.image.name", containerImageShorten),
			pmetrictest.ChangeResourceAttributeValue("container.image.tag", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.cronjob.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.daemonset.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.deployment.name", shortenNames),
			pmetrictest.ChangeResourceAttributeValue("k8s.deployment.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.hpa.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.job.name", shortenNames),
			pmetrictest.ChangeResourceAttributeValue("k8s.job.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.namespace.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.node.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.pod.name", shortenNames),
			pmetrictest.ChangeResourceAttributeValue("k8s.pod.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.name", shortenNames),
			pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.uid", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.statefulset.uid", replaceWithStar),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

func calculateCronJobExecution() time.Duration {
	// extract the number of second from the current timestamp
	seconds := time.Now().Second()
	// calculate the time until the full minute
	secondsToWait := 60 - seconds
	if secondsToWait <= 1 {
		secondsToWait = 60
	}

	return time.Duration(secondsToWait) * time.Second
}

func shortenNames(value string) string {
	if strings.HasPrefix(value, "coredns") {
		return "coredns"
	}
	if strings.HasPrefix(value, "kindnet") {
		return "kindnet"
	}
	if strings.HasPrefix(value, "kube-apiserver") {
		return "kube-apiserver"
	}
	if strings.HasPrefix(value, "kube-proxy") {
		return "kube-proxy"
	}
	if strings.HasPrefix(value, "kube-scheduler") {
		return "kube-scheduler"
	}
	if strings.HasPrefix(value, "kube-controller-manager") {
		return "kube-controller-manager"
	}
	if strings.HasPrefix(value, "local-path-provisioner") {
		return "local-path-provisioner"
	}
	if strings.HasPrefix(value, "otelcol") {
		return "otelcol"
	}
	if strings.HasPrefix(value, "test-k8scluster-receiver-cronjob") {
		return "test-k8scluster-receiver-cronjob"
	}
	if strings.HasPrefix(value, "test-k8scluster-receiver-job") {
		return "test-k8scluster-receiver-job"
	}
	return value
}

func replaceWithStar(_ string) string { return "*" }

func containerImageShorten(value string) string {
	// Extracts the image name by removing the repository prefix.
	// Also removes any architecture identifier suffix, if present, by applying shortenNames.
	return shortenNames(value[(strings.LastIndex(value, "/") + 1):])
}

func startUpSink(t *testing.T, consumer any) func() {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	cfg.HTTP = nil
	cfg.GRPC.NetAddr.Endpoint = "0.0.0.0:4317"

	var err error
	var rcvr component.Component

	switch c := consumer.(type) {
	case *consumertest.MetricsSink:
		rcvr, err = f.CreateMetrics(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, c)
	case *consumertest.LogsSink:
		rcvr, err = f.CreateLogs(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, c)
	default:
		t.Fatalf("unsupported consumer type: %T", c)
	}

	require.NoError(t, err, "failed creating receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))

	return func() {
		require.NoError(t, rcvr.Shutdown(context.Background()))
	}
}

func waitForData(t *testing.T, entriesNum int, consumer any) {
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		switch c := consumer.(type) {
		case *consumertest.MetricsSink:
			return len(c.AllMetrics()) > entriesNum
		case *consumertest.LogsSink:
			return len(c.AllLogs()) > entriesNum
		default:
			t.Fatalf("unsupported consumer type: %T", c)
			return false
		}
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries in %d minutes", entriesNum, timeoutMinutes)
}

// TestE2ENamespaceMetadata tests the k8s cluster receiver's exporting of entities in a real k8s cluster
func TestE2ENamespaceMetadata(t *testing.T) {
	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	logsConsumer := new(consumertest.LogsSink)
	shutdownSink := startUpSink(t, logsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "entities-test", "collector"), map[string]string{}, "")

	t.Cleanup(func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	})

	namespaceObj, err := k8stest.CreateObjects(k8sClient, filepath.Join(".", "testdata", "e2e", "entities-test", "testobjects"))
	require.NoErrorf(t, err, "failed to create test k8s objects")

	entityType := "k8s.namespace"
	entityNameKey := "k8s.namespace.name"
	entityName := "test-entities-ns"
	namespaceLogs := waitForEntityLogs(t, entityType, entityNameKey, entityName, logsConsumer)

	expected, err := golden.ReadLogs("./testdata/e2e/entities-test/expected-ns.yaml")
	require.NoError(t, err)

	commonReplacements := map[string]map[string]string{
		"otel.entity.attributes": {
			"k8s.namespace.creation_timestamp": "2025-01-01T00:00:00Z",
		},
		"otel.entity.id": {
			"k8s.namespace.uid": "entity-id",
		},
	}

	replaceLogValues(t, namespaceLogs[0], commonReplacements)

	require.NoError(t, plogtest.CompareLogs(expected, namespaceLogs[0],
		plogtest.IgnoreTimestamp(),
		plogtest.IgnoreObservedTimestamp(),
		plogtest.IgnoreScopeLogsOrder(),
		plogtest.IgnoreLogRecordsOrder(),
	))

	logsConsumer.Reset()

	// Delete test namespace object to trigger terminating phase and check if new event log is generated with the correct phase
	require.NoErrorf(t, k8stest.DeleteObjects(k8sClient, namespaceObj), "failed to delete test k8s objects")

	namespaceLogs = waitForEntityLogs(t, entityType, entityNameKey, entityName, logsConsumer)

	replaceLogValues(t, namespaceLogs[0], commonReplacements)

	// update the phase in expected log to terminating
	replaceLogValues(t, expected, map[string]map[string]string{
		"otel.entity.attributes": {
			"k8s.namespace.phase": "terminating",
		},
	})

	require.NoError(t, plogtest.CompareLogs(expected, namespaceLogs[0],
		plogtest.IgnoreTimestamp(),
		plogtest.IgnoreObservedTimestamp(),
		plogtest.IgnoreScopeLogsOrder(),
		plogtest.IgnoreLogRecordsOrder(),
	))
}

// filterEntityLogs returns logs that contain the entity with the given entityType and entityNameKey and entityName.
func filterEntityLogs(logs []plog.Logs, entityType, entityNameKey, entityName string) []plog.Logs {
	var entityLogs []plog.Logs
	for _, log := range logs {
		for i := 0; i < log.ResourceLogs().Len(); i++ {
			resourceLog := log.ResourceLogs().At(i)
			for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
				scopeLog := resourceLog.ScopeLogs().At(j)
				if containsEntity(scopeLog, entityType, entityNameKey, entityName) {
					entityLogs = append(entityLogs, log)
					break
				}
			}
		}
	}
	return entityLogs
}

// containsEntity returns true if the given scopeLog contains an entity with the given entityType, entityNameKey and entityName.
func containsEntity(scopeLog plog.ScopeLogs, entityType, entityNameKey, entityName string) bool {
	for k := 0; k < scopeLog.LogRecords().Len(); k++ {
		logRecord := scopeLog.LogRecords().At(k)
		entityTypeAttr, exists := logRecord.Attributes().Get("otel.entity.type")
		if exists && entityTypeAttr.Type() == pcommon.ValueTypeStr && entityTypeAttr.Str() == entityType {
			entityAttributesAttr, exists := logRecord.Attributes().Get("otel.entity.attributes")
			if exists && entityAttributesAttr.Type() == pcommon.ValueTypeMap {
				entityAttributes := entityAttributesAttr.Map()
				entityNameValue, exists := entityAttributes.Get(entityNameKey)
				if exists && entityNameValue.Type() == pcommon.ValueTypeStr && entityNameValue.Str() == entityName {
					return true
				}
			}
		}
	}
	return false
}

func waitForEntityLogs(t *testing.T, entityType, entityNameKey, entityName string, consumer *consumertest.LogsSink) []plog.Logs {
	timeoutMinutes := 3
	var entityLogs []plog.Logs
	require.Eventuallyf(t, func() bool {
		for _, logs := range consumer.AllLogs() {
			filteredLogs := filterEntityLogs([]plog.Logs{logs}, entityType, entityNameKey, entityName)
			if len(filteredLogs) > 0 {
				entityLogs = filteredLogs
				return true
			}
		}
		return false
	}, time.Duration(timeoutMinutes)*time.Minute, 5*time.Second,
		"failed to receive logs for entity %s with name %s in %d minutes", entityType, entityName, timeoutMinutes)
	return entityLogs
}

// replaceLogValue replaces the value of the key in the attribute with attributeName with the placeholder.
func replaceLogValue(logs plog.Logs, attributeName, key, placeholder string) error {
	rls := logs.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		sls := rls.At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				lr := lrs.At(k)
				attr, exists := lr.Attributes().Get(attributeName)
				if !exists {
					return fmt.Errorf("expected attribute %s not found in log record", attributeName)
				}
				if attr.Type() != pcommon.ValueTypeMap {
					return fmt.Errorf("attribute %s is not of type map in log record", attributeName)
				}
				attrMap := attr.Map()
				val, exists := attrMap.Get(key)
				if !exists {
					return fmt.Errorf("expected key %s not found in attribute %s map", key, attributeName)
				}
				val.SetStr(placeholder)
			}
		}
	}
	return nil
}

func replaceLogValues(t *testing.T, logs plog.Logs, replacements map[string]map[string]string) {
	for attributeName, keys := range replacements {
		for key, placeholder := range keys {
			err := replaceLogValue(logs, attributeName, key, placeholder)
			require.NoError(t, err)
		}
	}
}
