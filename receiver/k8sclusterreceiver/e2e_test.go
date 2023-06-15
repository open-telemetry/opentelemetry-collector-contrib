// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

package k8sclusterreceiver

import (
	"context"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"
)

const testKubeConfig = "/tmp/kube-config-otelcol-e2e-testing"

const shortenNamesRegex = "([\\D]+)-?.*"

var writeExpected = false

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
	collectorObjs := k8stest.CreateCollectorObjects(t, dynamicClient, testID)

	defer func() {
		for _, obj := range append(collectorObjs) {
			require.NoErrorf(t, k8stest.DeleteObject(dynamicClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	metricsConsumer := new(consumertest.MetricsSink)
	wantEntries := 10 // Minimal number of metrics to wait for.
	waitForData(t, wantEntries, metricsConsumer)

	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "expected.yaml")
	l := len(metricsConsumer.AllMetrics())
	if !writeExpected {
		expected, err = golden.ReadMetrics(expectedFile)
		require.NoError(t, err)
	} else {
		require.GreaterOrEqual(t, l, 1)
		err := golden.WriteMetrics(t, expectedFile, metricsConsumer.AllMetrics()[l-1])
		require.NoError(t, err)
		expected = metricsConsumer.AllMetrics()[l-1]
	}
	r, err := regexp.Compile(shortenNamesRegex)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[l-1],
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues("k8s.deployment.desired", "k8s.deployment.available", "k8s.container.restarts", "k8s.container.cpu_request", "k8s.container.memory_request", "k8s.container.memory_limit"),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.name", func(value string) string {
			matches := r.FindStringSubmatch(value)
			return matches[1]
		}),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.name", func(value string) string {
			matches := r.FindStringSubmatch(value)
			return matches[1]
		}),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.name", func(value string) string {
			matches := r.FindStringSubmatch(value)
			return matches[1]
		}),
		pmetrictest.IgnoreResourceAttributeValue("k8s.deployment.uid"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.replicaset.uid"),
		pmetrictest.IgnoreResourceAttributeValue("container.id"),
		pmetrictest.IgnoreResourceAttributeValue("container.image.tag"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.node.uid"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.namespace.uid"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.daemonset.uid"),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	),
	)
}

func TestRegex(t *testing.T) {
	strs := [][]string{
		{"otelcol-5ffb893c-5459b589fd", "otelcol-"},
		{"local-path-provisioner-684f458cdd-v726j", "local-path-provisioner-"},
		{"etcd-kind-control-plane", "etcd-kind-control-plane"},
		{"local-path-provisioner", "local-path-provisioner"},
	}
	for _, str := range strs {
		t.Run(str[0], func(t *testing.T) {
			r, err := regexp.Compile(shortenNamesRegex)
			require.NoError(t, err)
			matches := r.FindStringSubmatch(str[0])
			require.Equal(t, str[1], matches[1])
		})
	}
}

func waitForData(t *testing.T, entriesNum int, mc *consumertest.MetricsSink) {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)

	rcvr, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mc)
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating metrics receiver")
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d metrics in %d minutes", entriesNum,
		len(mc.AllMetrics()), timeoutMinutes)
}
