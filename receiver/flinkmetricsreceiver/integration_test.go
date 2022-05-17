// Copyright  The OpenTelemetry Authors
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

package flinkmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver"

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

var (
	metricsWithTaskmanagerID = []string{"flinkmetrics.operator.record.count", "flinkmetrics.task.record.count", "flinkmetrics.operator.watermark.output", "flinkmetrics.taskmanager.status.jvm.cpu.load", "flinkmetrics.taskmanager.status.jvm.cpu.time", "flinkmetrics.taskmanager.status.jvm.memory.heap.used", "flinkmetrics.taskmanager.status.jvm.memory.heap.committed", "flinkmetrics.taskmanager.status.jvm.memory.heap.max", "flinkmetrics.taskmanager.status.jvm.memory.non_heap.used", "flinkmetrics.taskmanager.status.jvm.memory.non_heap.committed", "flinkmetrics.taskmanager.status.jvm.memory.non_heap.max", "flinkmetrics.taskmanager.status.jvm.memory.metaspace.used", "flinkmetrics.taskmanager.status.jvm.memory.metaspace.committed", "flinkmetrics.taskmanager.status.jvm.memory.metaspace.max", "flinkmetrics.taskmanager.status.jvm.memory.direct.used", "flinkmetrics.taskmanager.status.jvm.memory.direct.total_capacity", "flinkmetrics.taskmanager.status.jvm.memory.mapped.used", "flinkmetrics.taskmanager.status.jvm.memory.mapped.total_capacity", "flinkmetrics.taskmanager.status.flink.memory.managed.used", "flinkmetrics.taskmanager.status.flink.memory.managed.total", "flinkmetrics.taskmanager.status.jvm.threads.count", "flinkmetrics.taskmanager.status.jvm.garbage_collector.collection.count", "flinkmetrics.taskmanager.status.jvm.garbage_collector.collection.time", "flinkmetrics.taskmanager.status.jvm.class_loader.classes_loaded"}
	metricsWithHost          = []string{"flinkmetrics.jobmanager.status.jvm.cpu.load", "flinkmetrics.taskmanager.status.jvm.cpu.load", "flinkmetrics.jobmanager.status.jvm.cpu.time", "flinkmetrics.taskmanager.status.jvm.cpu.time", "flinkmetrics.jobmanager.status.jvm.memory.heap.used", "flinkmetrics.taskmanager.status.jvm.memory.heap.used", "flinkmetrics.jobmanager.status.jvm.memory.heap.committed", "flinkmetrics.taskmanager.status.jvm.memory.heap.committed", "flinkmetrics.jobmanager.status.jvm.memory.heap.max", "flinkmetrics.taskmanager.status.jvm.memory.heap.max", "flinkmetrics.jobmanager.status.jvm.memory.non_heap.used", "flinkmetrics.taskmanager.status.jvm.memory.non_heap.used", "flinkmetrics.jobmanager.status.jvm.memory.non_heap.committed", "flinkmetrics.taskmanager.status.jvm.memory.non_heap.committed", "flinkmetrics.jobmanager.status.jvm.memory.non_heap.max", "flinkmetrics.taskmanager.status.jvm.memory.non_heap.max", "flinkmetrics.jobmanager.status.jvm.memory.metaspace.used", "flinkmetrics.taskmanager.status.jvm.memory.metaspace.used", "flinkmetrics.jobmanager.status.jvm.memory.metaspace.committed", "flinkmetrics.taskmanager.status.jvm.memory.metaspace.committed", "flinkmetrics.jobmanager.status.jvm.memory.metaspace.max", "flinkmetrics.taskmanager.status.jvm.memory.metaspace.max", "flinkmetrics.jobmanager.status.jvm.memory.direct.used", "flinkmetrics.taskmanager.status.jvm.memory.direct.used", "flinkmetrics.jobmanager.status.jvm.memory.direct.total_capacity", "flinkmetrics.taskmanager.status.jvm.memory.direct.total_capacity", "flinkmetrics.jobmanager.status.jvm.memory.mapped.used", "flinkmetrics.taskmanager.status.jvm.memory.mapped.used", "flinkmetrics.jobmanager.status.jvm.memory.mapped.total_capacity", "flinkmetrics.taskmanager.status.jvm.memory.mapped.total_capacity", "flinkmetrics.jobmanager.status.flink.memory.managed.used", "flinkmetrics.taskmanager.status.flink.memory.managed.used", "flinkmetrics.jobmanager.status.flink.memory.managed.total", "flinkmetrics.taskmanager.status.flink.memory.managed.total", "flinkmetrics.jobmanager.status.jvm.threads.count", "flinkmetrics.taskmanager.status.jvm.threads.count", "flinkmetrics.jobmanager.status.jvm.garbage_collector.collection.count", "flinkmetrics.taskmanager.status.jvm.garbage_collector.collection.count", "flinkmetrics.jobmanager.status.jvm.garbage_collector.collection.time", "flinkmetrics.taskmanager.status.jvm.garbage_collector.collection.time", "flinkmetrics.jobmanager.status.jvm.class_loader.classes_loaded", "flinkmetrics.taskmanager.status.jvm.class_loader.classes_loaded", "flinkmetrics.job.restart.count", "flinkmetrics.job.last_checkpoint.time", "flinkmetrics.job.last_checkpoint.size", "flinkmetrics.job.checkpoints.count", "flinkmetrics.task.record.count", "flinkmetrics.operator.record.count", "flinkmetrics.operator.watermark.output"}
)

func TestFlinkIntegration(t *testing.T) {
	t.Parallel()
	networkName := "new-network"
	ctx := context.Background()
	newNetwork, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name:           networkName,
			CheckDuplicate: true,
		},
	})
	if err != nil {
		require.NoError(t, err)
	}
	defer newNetwork.Remove(ctx)

	masterContainer := getContainer(t, testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    path.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.flink-master",
		},
		Hostname:     "flink-master",
		Networks:     []string{networkName},
		ExposedPorts: []string{"8080:8080", "8081:8081"},
		WaitingFor:   waitStrategy{endpoint: "http://localhost:8081/jobmanager/metrics"},
	})

	workerContainer := getContainer(t, testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    path.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.flink-worker",
		},
		Hostname: "worker",
		Networks: []string{networkName},
	})
	defer func() {
		require.NoError(t, masterContainer.Terminate(context.Background()))
	}()
	defer func() {
		require.NoError(t, workerContainer.Terminate(context.Background()))
	}()

	hostname, err := masterContainer.Host(context.Background())
	require.NoError(t, err)

	// Required to start the taskmanager
	ws := waitStrategy{"http://localhost:8081/taskmanagers/metrics"}
	err = ws.waitFor(context.Background(), "")
	require.NoError(t, err)

	// required to start the StateMachineExample job
	code, err := masterContainer.Exec(context.Background(), []string{"/setup.sh"})
	require.NoError(t, err)
	require.Equal(t, 0, code)

	// Required to prevent empty value jobs call
	ws = waitStrategy{endpoint: "http://localhost:8081/jobs"}
	err = ws.waitFor(context.Background(), "")
	require.NoError(t, err)

	// Required to prevent empty value for job, operator and task metrics call
	ws = waitStrategy{endpoint: "http://localhost:8081/jobs/metrics"}
	err = ws.waitFor(context.Background(), "")
	require.NoError(t, err)

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.ScraperControllerSettings.CollectionInterval = 100 * time.Millisecond
	cfg.Endpoint = fmt.Sprintf("http://%s", net.JoinHostPort(hostname, "8081"))

	consumer := new(consumertest.MetricsSink)
	settings := componenttest.NewNopReceiverCreateSettings()
	rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.Eventuallyf(t, func() bool {
		return consumer.DataPointCount() > 0
	}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
	require.NoError(t, rcvr.Shutdown(context.Background()))

	actualMetrics := consumer.AllMetrics()[0]

	expectedFile := filepath.Join("testdata", "integration", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	// tmp
	err = golden.WriteMetrics(expectedFile, actualMetrics)
	require.NoError(t, err)
	// tmp

	ignoreTaskmanagerIDAttributeValues := scrapertest.IgnoreMetricAttributeValue("taskmanager_id", metricsWithTaskmanagerID...)
	ignoreHostAttributeValues := scrapertest.IgnoreMetricAttributeValue("host", metricsWithHost...)
	// ignoreTaskmanagerIDAttributeValues := scrapertest.IgnoreMetricAttributeValue(metadata.A.TaskmanagerID, metricsWithTaskmanagerID...)
	// ignoreHostAttributeValues := scrapertest.IgnoreMetricAttributeValue(metadata.A.Host, metricsWithHost...)

	// require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues(), ignoreTaskmanagerIDAttributeValues))
	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues(), ignoreTaskmanagerIDAttributeValues, ignoreHostAttributeValues))
}

func getContainer(t *testing.T, req testcontainers.ContainerRequest) testcontainers.Container {
	require.NoError(t, req.Validate())
	container, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	require.NoError(t, err)
	return container
}

type waitStrategy struct {
	endpoint string
}

func (ws waitStrategy) WaitUntilReady(ctx context.Context, st wait.StrategyTarget) error {
	if err := wait.ForListeningPort("8081").
		WithStartupTimeout(2*time.Minute).
		WaitUntilReady(ctx, st); err != nil {
		return err
	}

	hostname, err := st.Host(ctx)
	if err != nil {
		return err
	}

	return ws.waitFor(ctx, hostname)
}

func (ws waitStrategy) waitFor(ctx context.Context, hostname string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return fmt.Errorf("server startup problem")
		case <-time.After(100 * time.Millisecond):
			resp, err := http.Get(ws.endpoint)
			if err != nil {
				continue
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				continue
			}

			if resp.Body.Close() != nil {
				continue
			}

			// The server needs a moment to generate some stats
			if strings.Contains(string(body), "id") {
				return nil
			}
		}
	}
}
