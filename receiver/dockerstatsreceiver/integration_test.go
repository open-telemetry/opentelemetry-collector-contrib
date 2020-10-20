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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/translator/conventions"
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

func paramsAndContext(t *testing.T) (component.ReceiverCreateParams, context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))
	return component.ReceiverCreateParams{Logger: logger}, ctx, cancel
}

func TestDefaultMetricsIntegration(t *testing.T) {
	params, ctx, cancel := paramsAndContext(t)
	defer cancel()
	d := container.New(t)
	d.StartImage("docker.io/library/nginx:1.17", container.WithPortReady(80))

	consumer := new(consumertest.MetricsSink)
	f, config := factory()
	receiver, err := f.CreateMetricsReceiver(ctx, params, config, consumer)
	r := receiver.(*Receiver)

	require.NoError(t, err, "failed creating metrics Receiver")
	require.NoError(t, r.Start(ctx, &testHost{
		t: t,
	}))

	assert.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 5*time.Second, 1*time.Second, "failed to receive any metrics")

	assert.NoError(t, r.Shutdown(ctx))
}

func TestAllMetricsIntegration(t *testing.T) {
	d := container.New(t)
	d.StartImage("docker.io/library/nginx:1.17", container.WithPortReady(80))

	consumer := new(consumertest.MetricsSink)
	f, config := factory()
	config.ProvidePerCoreCPUMetrics = true

	params, ctx, cancel := paramsAndContext(t)
	defer cancel()

	receiver, err := f.CreateMetricsReceiver(ctx, params, config, consumer)
	r := receiver.(*Receiver)

	require.NoError(t, err, "failed creating metrics Receiver")
	require.NoError(t, r.Start(ctx, &testHost{
		t: t,
	}))

	assert.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 5*time.Second, 1*time.Second, "failed to receive any metrics")

	assert.NoError(t, r.Shutdown(ctx))
}

func TestMonitoringAddedContainerIntegration(t *testing.T) {
	params, ctx, cancel := paramsAndContext(t)
	defer cancel()
	consumer := new(consumertest.MetricsSink)
	f, config := factory()

	receiver, err := f.CreateMetricsReceiver(ctx, params, config, consumer)
	r := receiver.(*Receiver)

	require.NoError(t, err, "failed creating metrics Receiver")
	require.NoError(t, r.Start(ctx, &testHost{
		t: t,
	}))

	d := container.New(t)
	d.StartImage("docker.io/library/nginx:1.17", container.WithPortReady(80))

	assert.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 5*time.Second, 1*time.Second, "failed to receive any metrics")

	assert.NoError(t, r.Shutdown(ctx))
}

func TestExcludedImageProducesNoMetricsIntegration(t *testing.T) {
	params, ctx, cancel := paramsAndContext(t)
	defer cancel()
	d := container.New(t)
	d.StartImage("docker.io/library/redis:6.0.3", container.WithPortReady(6379))

	f, config := factory()
	config.ExcludedImages = append(config.ExcludedImages, "*redis*")

	consumer := new(consumertest.MetricsSink)
	receiver, err := f.CreateMetricsReceiver(ctx, params, config, consumer)
	r := receiver.(*Receiver)

	require.NoError(t, err, "failed creating metrics Receiver")
	require.NoError(t, r.Start(ctx, &testHost{
		t: t,
	}))

	assert.Never(t, func() bool {
		if metrics := consumer.AllMetrics(); len(metrics) > 0 {
			for _, metric := range metrics {
				resourceMetrics := metric.ResourceMetrics()
				for i := 0; i < resourceMetrics.Len(); i++ {
					resourceMetric := resourceMetrics.At(i)
					resource := resourceMetric.Resource()
					nameAttr, ok := resource.Attributes().Get(conventions.AttributeContainerImage)
					if ok && !nameAttr.IsNil() {
						if strings.Contains(nameAttr.StringVal(), "redis") {
							return true
						}
					}
				}
			}
		}
		return false
	}, 5*time.Second, 1*time.Second, "received undesired metrics")

	assert.NoError(t, r.Shutdown(ctx))
}

func TestRemovedContainerRemovesRecordsIntegration(t *testing.T) {
	params, ctx, cancel := paramsAndContext(t)
	defer cancel()
	consumer := new(consumertest.MetricsSink)
	f, config := factory()
	config.ExcludedImages = append(config.ExcludedImages, "!*nginx*")
	receiver, err := f.CreateMetricsReceiver(ctx, params, config, consumer)
	r := receiver.(*Receiver)

	require.NoError(t, r.Start(ctx, &testHost{
		t: t,
	}))

	d := container.New(t)
	nginx := d.StartImage("docker.io/library/nginx:1.17", container.WithPortReady(80))

	require.NoError(t, err, "failed creating metrics Receiver")

	desiredAmount := func(numDesired int) func() bool {
		return func() bool {
			// We need the lock to prevent data race warnings for test activity
			r.client.containersLock.Lock()
			defer r.client.containersLock.Unlock()
			return len(r.client.containers) == numDesired
		}
	}

	assert.Eventuallyf(t, desiredAmount(1), 5*time.Second, 1*time.Millisecond, "failed to load container stores")
	containers := r.client.Containers()
	d.RemoveContainer(nginx)
	assert.Eventuallyf(t, desiredAmount(0), 5*time.Second, 1*time.Millisecond, "failed to clear container stores")

	// Confirm missing container paths
	md, err := r.client.FetchContainerStatsAndConvertToMetrics(ctx, containers[0])
	assert.Nil(t, md)
	require.Error(t, err)
	assert.Equal(t, fmt.Sprintf("Error response from daemon: No such container: %s", containers[0].ID), err.Error())

	ctr, ok := r.client.inspectedContainerIsOfInterest(ctx, containers[0].ID)
	assert.False(t, ok)
	assert.Nil(t, ctr)

	assert.NoError(t, r.Shutdown(ctx))
}
