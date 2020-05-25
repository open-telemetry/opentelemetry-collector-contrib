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

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/testutils"
)

func TestReceiver(t *testing.T) {
	client := fake.NewSimpleClientset()
	consumer := &exportertest.SinkMetricsExporterOld{}

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
	expectedResources := numPods + numNodes
	require.Eventually(t, func() bool {
		return len(r.resourceWatcher.dataCollector.CollectMetricData()) == expectedResources
	}, 10*time.Second, 100*time.Millisecond,
		"metrics not collected")

	numPodsToDelete := 1
	deletePods(t, client, numPodsToDelete)

	// Expects metric data from a node, since other resources were deleted.
	expectedResources = (numPods - numPodsToDelete) + numNodes
	require.Eventually(t, func() bool {
		return len(r.resourceWatcher.dataCollector.CollectMetricData()) == expectedResources
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
	createPods(t, client, numPods)

	ctx := context.Background()
	r.Start(ctx, componenttest.NewNopHost())

	require.Eventually(t, func() bool {
		return len(consumer.AllMetrics()) == numPods
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

	rw, err := newResourceWatcher(logger, config, client)

	if err != nil {
		return nil, err
	}

	rw.dataCollector.SetupMetadataStore(&corev1.Service{}, &testutils.MockStore{})

	return &kubernetesReceiver{
		resourceWatcher: rw,
		logger:          logger,
		config:          config,
		consumer:        consumer,
	}, nil
}

func createPods(t *testing.T, client *fake.Clientset, numPods int) {
	for i := 0; i < numPods; i++ {
		p := &corev1.Pod{
			ObjectMeta: v1.ObjectMeta{
				UID:       types.UID("pod" + strconv.Itoa(i)),
				Name:      strconv.Itoa(i),
				Namespace: "test",
			},
		}
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
		err := client.CoreV1().Pods("test").Delete(strconv.Itoa(i), &v1.DeleteOptions{})

		if err != nil {
			t.Errorf("error deleting pod: %v", err)
			t.FailNow()
		}
	}

	time.Sleep(2 * time.Millisecond)
}

func createNodes(t *testing.T, client *fake.Clientset, numNodes int) {
	for i := 0; i < numNodes; i++ {
		n := &corev1.Node{
			ObjectMeta: v1.ObjectMeta{
				UID:  types.UID("node" + strconv.Itoa(i)),
				Name: strconv.Itoa(i),
			},
		}
		_, err := client.CoreV1().Nodes().Create(n)

		if err != nil {
			t.Errorf("error creating node: %v", err)
			t.FailNow()
		}

		time.Sleep(2 * time.Millisecond)
	}
}
