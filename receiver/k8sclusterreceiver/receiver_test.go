// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sclusterreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/testutils"
)

func TestReceiver(t *testing.T) {
	client := fake.NewSimpleClientset()
	consumer := &exportertest.SinkMetricsExporter{}

	r, err := setupReceiver(client, consumer)

	require.NoError(t, err)

	// Setup k8s resources.
	numPods := 2
	numNodes := 1
	createPods(t, client, numPods)
	createNodes(t, client, numNodes)

	ctx := context.Background()
	r.Start(ctx, componenttest.NewNopHost())

	// Expects metric data from nodes and pods where each metric data
	// struct corresponds to one resource.
	expectedNumMetrics := numPods + numNodes
	var initialMetricsCount int
	require.Eventually(t, func() bool {
		initialMetricsCount = consumer.MetricsCount()
		return initialMetricsCount == expectedNumMetrics
	}, 10*time.Second, 100*time.Millisecond,
		"metrics not collected")

	numPodsToDelete := 1
	deletePods(t, client, numPodsToDelete)

	// Expects metric data from a node, since other resources were deleted.
	expectedNumMetrics = (numPods - numPodsToDelete) + numNodes
	var metricsCountDelta int
	require.Eventually(t, func() bool {
		metricsCountDelta = consumer.MetricsCount() - initialMetricsCount
		return metricsCountDelta == expectedNumMetrics
	}, 10*time.Second, 100*time.Millisecond,
		"updated metrics not collected")

	r.Shutdown(ctx)
}

func TestReceiverWithManyResources(t *testing.T) {
	client := fake.NewSimpleClientset()
	consumer := &exportertest.SinkMetricsExporter{}

	r, err := setupReceiver(client, consumer)

	require.NoError(t, err)

	numPods := 1000
	createPods(t, client, numPods)

	ctx := context.Background()
	r.Start(ctx, componenttest.NewNopHost())

	require.Eventually(t, func() bool {
		return consumer.MetricsCount() == numPods
	}, 10*time.Second, 100*time.Millisecond,
		"metrics not collected")

	r.Shutdown(ctx)
}

func setupReceiver(
	client *fake.Clientset,
	consumer consumer.MetricsConsumer) (*kubernetesReceiver, error) {

	logger := zap.NewNop()
	rOptions := &receiverOptions{
		collectionInterval:         1 * time.Second,
		nodeConditionTypesToReport: []string{"Ready"},
		client:                     client,
	}

	rw, err := newResourceWatcher(logger, rOptions)

	if err != nil {
		return nil, err
	}

	rw.dataCollector.SetupMetadataStore(&corev1.Service{}, &testutils.MockStore{})

	return &kubernetesReceiver{
		resourceWatcher: rw,
		logger:          logger,
		options:         rOptions,
		consumer:        consumer,
	}, nil
}
