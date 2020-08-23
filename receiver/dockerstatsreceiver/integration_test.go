// Copyright The OpenTelemetry Authors
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

// +build integration

package dockerstatsreceiver

import (
	"context"
	"testing"
	"time"

	dtypes "github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testing/container"
)

type testHost struct {
	component.Host
	t *testing.T
}

// ReportFatalError causes the test to be run to fail.
func (h *testHost) ReportFatalError(err error) {
	h.t.Fatalf("Receiver reported a fatal error: %v", err)
}

var _ component.Host = (*testHost)(nil)

func factory() (component.ReceiverFactory, *Config) {
	f := NewFactory()
	config := f.CreateDefaultConfig().(*Config)
	config.CollectionInterval = 1 * time.Second
	return f, config
}

func params(t *testing.T) component.ReceiverCreateParams {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))
	return component.ReceiverCreateParams{Logger: logger}
}

func TestDefaultMetricsIntegration(t *testing.T) {
	params := params(t)
	d := container.New(t)
	d.StartImage("docker.io/library/nginx:1.17", container.WithPortReady(80))

	consumer := &exportertest.SinkMetricsExporter{}
	f, config := factory()
	receiver, err := f.CreateMetricsReceiver(context.Background(), params, config, consumer)
	r := receiver.(*Receiver)

	require.NoError(t, err, "failed creating metrics Receiver")
	require.NoError(t, r.Start(context.Background(), &testHost{
		t: t,
	}))

	assert.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 5*time.Second, 1*time.Second, "failed to receive any metrics")

	assert.NoError(t, r.Shutdown(context.Background()))
}

func TestAllMetricsIntegration(t *testing.T) {
	params := params(t)
	d := container.New(t)
	d.StartImage("docker.io/library/nginx:1.17", container.WithPortReady(80))

	consumer := &exportertest.SinkMetricsExporter{}
	f, config := factory()

	config.ProvidePerCoreCPUMetrics = true

	receiver, err := f.CreateMetricsReceiver(context.Background(), params, config, consumer)
	r := receiver.(*Receiver)

	require.NoError(t, err, "failed creating metrics Receiver")
	require.NoError(t, r.Start(context.Background(), &testHost{
		t: t,
	}))

	assert.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 5*time.Second, 1*time.Second, "failed to receive any metrics")

	assert.NoError(t, r.Shutdown(context.Background()))
}

func TestExcludedImageProducesNoMetricsIntegration(t *testing.T) {
	params := params(t)
	d := container.New(t)
	d.StartImage("docker.io/library/redis:6.0.3", container.WithPortReady(6379))

	f, config := factory()
	config.ExcludedImages = append(config.ExcludedImages, "*redis*")

	consumer := &exportertest.SinkMetricsExporter{}
	receiver, err := f.CreateMetricsReceiver(context.Background(), params, config, consumer)
	r := receiver.(*Receiver)

	require.NoError(t, err, "failed creating metrics Receiver")
	require.NoError(t, r.Start(context.Background(), &testHost{
		t: t,
	}))

	assert.Never(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 5*time.Second, 1*time.Second, "received undesired metrics")

	assert.NoError(t, r.Shutdown(context.Background()))
}

func TestFetchMissingContainerIntegration(t *testing.T) {
	params := params(t)
	consumer := &exportertest.SinkMetricsExporter{}
	f, config := factory()
	receiver, err := f.CreateMetricsReceiver(context.Background(), params, config, consumer)
	require.NoError(t, err, "failed creating metrics Receiver")
	r := receiver.(*Receiver)
	require.NoError(t, r.Start(context.Background(), &testHost{
		t: t,
	}))

	container := DockerContainer{
		ContainerJSON: &dtypes.ContainerJSON{
			ContainerJSONBase: &dtypes.ContainerJSONBase{
				ID: "notADockerContainer",
			},
		},
	}

	md, err := r.client.FetchContainerStatsAndConvertToMetrics(context.Background(), container)
	assert.Nil(t, md)
	require.Error(t, err)
	assert.Equal(t, "Error response from daemon: No such container: notADockerContainer", err.Error())
}
