// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package mqttexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestIntegration(t *testing.T) {
	// Skip integration tests if not running with integration tag
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start MQTT broker container
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "eclipse-mosquitto:2.0",
		ExposedPorts: []string{"1883/tcp"},
<<<<<<< HEAD
		SkipReaper:   false,
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      "testdata/mosquitto.conf",
				ContainerFilePath: "/mosquitto/config/mosquitto.conf",
				FileMode:          0644,
			},
		},
		WaitingFor: wait.ForLog("mosquitto version 2.0.22 running"),
=======
		WaitingFor:   wait.ForLog("mosquitto version 2.0 running"),
>>>>>>> 2c2e65cb1a (feat(mqttexporter): add MQTT exporter and wire into local build)
	}

	mqttContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	defer func() {
<<<<<<< HEAD
		if mqttContainer != nil {
			assert.NoError(t, mqttContainer.Terminate(ctx))
		}
	}()

	ip, err := mqttContainer.ContainerIP(ctx)
	require.NoError(t, err)
	port, err := mqttContainer.MappedPort(ctx, "1883")
	require.NoError(t, err)
	t.Logf("MQTT broker running at %s:%s", ip, port.Port())
=======
		assert.NoError(t, mqttContainer.Terminate(ctx))
	}()

	// Get the host and port
	host, err := mqttContainer.Host(ctx)
	require.NoError(t, err)
	port, err := mqttContainer.MappedPort(ctx, "1883")
	require.NoError(t, err)
>>>>>>> 2c2e65cb1a (feat(mqttexporter): add MQTT exporter and wire into local build)

	// Create exporter configuration
	cfg := &Config{
		Connection: ConnectionConfig{
<<<<<<< HEAD
			Endpoint: "tcp://" + ip + ":" + "1883",
			ConnectionTimeout: 10 * time.Second,
			KeepAlive:         30 * time.Second,
			PublishConfirmationTimeout: 5 * time.Second,
=======
			Endpoint: "tcp://" + host + ":" + port.Port(),
			Auth: AuthConfig{
				Plain: PlainAuth{
					Username: "test",
					Password: "test",
				},
			},
			ConnectionTimeout: 10 * time.Second,
			KeepAlive:         30 * time.Second,
>>>>>>> 2c2e65cb1a (feat(mqttexporter): add MQTT exporter and wire into local build)
		},
		Topic: TopicConfig{
			Topic: "test/telemetry",
		},
		QoS:    1,
		Retain: false,
	}
<<<<<<< HEAD
=======

>>>>>>> 2c2e65cb1a (feat(mqttexporter): add MQTT exporter and wire into local build)
	// Create exporter
	set := exportertest.NewNopSettings(metadata.Type)
	exporter, err := createTracesExporter(ctx, set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)
<<<<<<< HEAD
=======

>>>>>>> 2c2e65cb1a (feat(mqttexporter): add MQTT exporter and wire into local build)
	// Start exporter
	hostComponent := componenttest.NewNopHost()
	err = exporter.Start(ctx, hostComponent)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, exporter.Shutdown(ctx))
	}()
<<<<<<< HEAD
	t.Log("Testing publishing traces...")
=======

>>>>>>> 2c2e65cb1a (feat(mqttexporter): add MQTT exporter and wire into local build)
	// Test publishing traces
	traces := testdata.GenerateTracesOneSpan()
	err = exporter.ConsumeTraces(ctx, traces)
	require.NoError(t, err)

	// Test publishing metrics
	metricsExporter, err := createMetricsExporter(ctx, set, cfg)
	require.NoError(t, err)
	require.NotNil(t, metricsExporter)

	err = metricsExporter.Start(ctx, hostComponent)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, metricsExporter.Shutdown(ctx))
	}()

	metrics := testdata.GenerateMetricsOneMetric()
	err = metricsExporter.ConsumeMetrics(ctx, metrics)
	require.NoError(t, err)

	// Test publishing logs
	logsExporter, err := createLogsExporter(ctx, set, cfg)
	require.NoError(t, err)
	require.NotNil(t, logsExporter)

	err = logsExporter.Start(ctx, hostComponent)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, logsExporter.Shutdown(ctx))
	}()

	logs := testdata.GenerateLogsOneLogRecord()
	err = logsExporter.ConsumeLogs(ctx, logs)
	require.NoError(t, err)
}
