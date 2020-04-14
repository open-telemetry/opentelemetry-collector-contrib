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
	"strconv"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector/component/componenttest"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exportertest"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func TestReceiver(t *testing.T) {
	client := fake.NewSimpleClientset()
	consumer := &exportertest.SinkMetricsExporterOld{}

	r, err := setupReceiver(client, consumer)

	require.NoError(t, err)

	// Setup k8s resources.
	numPods := 2
	numNodes := 1
	createPodsWithContainer(t, client, numPods)
	createNodes(t, client, numNodes)

	ctx := context.Background()
	r.Start(ctx, componenttest.NewNopHost())

	// Expects metric data from nodes, pods and containers where
	// each metric data struct corresponds to one resource.
	expectedResources := 2*numPods + numNodes
	require.Eventually(t, func() bool {
		return len(r.resourceWatcher.dataCollector.collectMetrics()) == expectedResources
	}, 10*time.Second, 100*time.Millisecond,
		"metrics not collected")

	numPodsToDelete := 1
	deletePods(t, client, numPodsToDelete)

	// Expects metric data from a node, since other resources were deleted.
	expectedResources = 2*(numPods-numPodsToDelete) + numNodes
	require.Eventually(t, func() bool {
		return len(r.resourceWatcher.dataCollector.collectMetrics()) == expectedResources
	}, 10*time.Second, 100*time.Millisecond,
		"updated metrics not collected")

	r.Shutdown(ctx)
}

func TestReceiverWithManyResources(t *testing.T) {
	client := fake.NewSimpleClientset()
	consumer := &exportertest.SinkMetricsExporterOld{}

	r, err := setupReceiver(client, consumer)

	require.NoError(t, err)

	numPods := 1000
	createPodsWithContainer(t, client, numPods)

	ctx := context.Background()
	r.Start(ctx, componenttest.NewNopHost())

	// Expects metric data from pods and containers where
	// each metric data struct corresponds to one resource.
	expectedResources := 2 * numPods
	require.Eventually(t, func() bool {
		return len(consumer.AllMetrics()) == expectedResources
	}, 10*time.Second, 100*time.Millisecond,
		"metrics not collected")

	r.Shutdown(ctx)
}

func setupReceiver(client *fake.Clientset,
	consumer consumer.MetricsConsumerOld) (*kubernetesReceiver, error) {

	logger := zap.NewNop()
	config := &Config{
		CollectionInterval:         1 * time.Second,
		NodeConditionTypesToReport: []string{"Ready"},
	}

	rw, err := newResourceWatcher(logger, config, client, true)

	if err != nil {
		return nil, err
	}

	rw.dataCollector.metadataStore = &metadataStore{
		map[string]cache.Store{
			"Service": &MockStore{},
		},
	}

	return &kubernetesReceiver{
		resourceWatcher: rw,
		logger:          logger,
		config:          config,
		consumer:        consumer,
	}, nil
}

func createPodsWithContainer(t *testing.T, client *fake.Clientset, numPods int) {
	for i := 0; i < numPods; i++ {
		p := newPodWithContainer(strconv.Itoa(i))
		_, err := client.CoreV1().Pods(p.Namespace).Create(p)

		if err != nil {
			t.Errorf("error creating pod: %v", err)
			t.FailNow()
		}

		time.Sleep(2 * time.Millisecond)
	}
}

func deletePods(t *testing.T, client *fake.Clientset, numPods int) {
	for i := 0; i < numPods; i++ {
		err := client.CoreV1().Pods("test-namespace").Delete("test-pod-"+strconv.Itoa(i), &v1.DeleteOptions{})

		if err != nil {
			t.Errorf("error deleting pod: %v", err)
			t.FailNow()
		}
	}

	time.Sleep(2 * time.Millisecond)
}

func createNodes(t *testing.T, client *fake.Clientset, numNodes int) {
	for i := 0; i < numNodes; i++ {
		n := newNode(strconv.Itoa(i))
		_, err := client.CoreV1().Nodes().Create(n)

		if err != nil {
			t.Errorf("error creating node: %v", err)
			t.FailNow()
		}

		time.Sleep(2 * time.Millisecond)
	}
}
