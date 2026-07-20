// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package rabbitmqexporter

import (
	"fmt"
	"testing"
	"time"

	"github.com/moby/moby/client"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

const (
	username = "myuser"
	password = "mypass"
	vhost    = "myvhost"
)

func TestExportWithNetworkIssueRecovery(t *testing.T) {
	testCase := []struct {
		name  string
		image string
	}{
		{
			name:  "test rabbitmq 4.2",
			image: "rabbitmq:4.2",
		},
	}

	for _, c := range testCase {
		t.Run(c.name, func(t *testing.T) {
			container := startRabbitMQContainer(t, c.image)
			defer func() {
				err := container.Terminate(t.Context())
				require.NoError(t, err)
			}()

			// Connect to rabbitmq then create a queue and queue consumer
			host, err := container.Host(t.Context())
			require.NoError(t, err)
			mappedPort, err := container.MappedPort(t.Context(), "5672")
			require.NoError(t, err)
			endpoint := fmt.Sprintf("amqp://%s:%s", host, mappedPort.Port())
			connection, channel, consumer := setupQueueConsumer(t, logsRoutingKey, endpoint)

			// Create and start rabbitmqexporter
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Connection.Endpoint = endpoint
			cfg.Connection.VHost = vhost
			cfg.Connection.Auth = AuthConfig{Plain: PlainAuth{Username: username, Password: password}}
			exporter, err := factory.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
			require.NoError(t, err)
			err = exporter.Start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				err = exporter.Shutdown(t.Context())
				require.NoError(t, err)
			}()

			// Export and verify data is consumed
			logs := testdata.GenerateLogsOneLogRecord()
			err = exporter.ConsumeLogs(t.Context(), logs)
			require.NoError(t, err)
			consumed := <-consumer
			unmarshaller := &plog.ProtoUnmarshaler{}
			receivedLogs, err := unmarshaller.UnmarshalLogs(consumed.Body)
			require.NoError(t, err)
			require.Equal(t, logs, receivedLogs)

			// Pause the container before exporting the next logs to simulate a network issue
			err = channel.Close()
			require.NoError(t, err)
			err = connection.Close()
			require.NoError(t, err)

			// Use Docker client to pause the container
			dockerClient, err := client.New(client.FromEnv)
			require.NoError(t, err)
			defer dockerClient.Close()

			_, err = dockerClient.ContainerPause(t.Context(), container.GetContainerID(), client.ContainerPauseOptions{})
			require.NoError(t, err)
			logs = testdata.GenerateLogsOneLogRecord()
			err = exporter.ConsumeLogs(t.Context(), logs)
			require.Error(t, err)

			// Unpause container to simulate network issue recovery
			_, err = dockerClient.ContainerUnpause(t.Context(), container.GetContainerID(), client.ContainerUnpauseOptions{})
			require.NoError(t, err)
			connection, channel, consumer = setupQueueConsumer(t, logsRoutingKey, endpoint)
			defer func() {
				channel.Close()
				connection.Close()
			}()

			logs = testdata.GenerateLogsOneLogRecord()
			err = exporter.ConsumeLogs(t.Context(), logs)
			require.NoError(t, err)
			consumed = <-consumer
			receivedLogs, err = unmarshaller.UnmarshalLogs(consumed.Body)
			require.NoError(t, err)
			require.Equal(t, logs, receivedLogs)
		})
	}
}

func startRabbitMQContainer(t *testing.T, image string) testcontainers.Container {
	container, err := testcontainers.GenericContainer(
		t.Context(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        image,
				ExposedPorts: []string{"5672/tcp"},
				WaitingFor: &wait.MultiStrategy{
					Strategies: []wait.Strategy{
						wait.ForListeningPort("5672").WithStartupTimeout(1 * time.Minute),
						wait.ForExec([]string{"rabbitmq-diagnostics", "check_running"}).WithStartupTimeout(1 * time.Minute),
					},
				},
				Env: map[string]string{
					"RABBITMQ_DEFAULT_USER":  username,
					"RABBITMQ_DEFAULT_PASS":  password,
					"RABBITMQ_DEFAULT_VHOST": vhost,
				},
			},
			Started: true,
		})
	require.NoError(t, err)

	err = container.Start(t.Context())
	require.NoError(t, err)
	return container
}

func setupQueueConsumer(t *testing.T, queueName, endpoint string) (*amqp.Connection, *amqp.Channel, <-chan amqp.Delivery) {
	connection, err := amqp.DialConfig(endpoint, amqp.Config{
		SASL: []amqp.Authentication{
			&amqp.PlainAuth{
				Username: username,
				Password: password,
			},
		},
		Vhost: vhost,
	})
	require.NoError(t, err)

	channel, err := connection.Channel()
	require.NoError(t, err)

	queue, err := channel.QueueDeclare(queueName, false, false, false, false, nil)
	require.NoError(t, err)

	consumer, err := channel.Consume(queue.Name, "testconsumer", true, false, false, false, nil)
	require.NoError(t, err)

	return connection, channel, consumer
}
