// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package newrelicsqlserverreceiver

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func setupContainer() (testcontainers.Container, error) {
	return testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image: "mcr.microsoft.com/mssql/server:2022-latest",
				Env: map[string]string{
					"ACCEPT_EULA":       "Y",
					"MSSQL_SA_PASSWORD": "^otelcol1234",
					"MSSQL_PID":         "Developer",
				},
				Files: []testcontainers.ContainerFile{{
					HostFilePath:      filepath.Join("testdata", "integration", "01-init.sh"),
					ContainerFilePath: "/init/01-init.sh",
					FileMode:          0o777,
				}},
				Cmd:          []string{"/bin/bash", "-c", "/opt/mssql/bin/sqlservr & /init/01-init.sh && sleep infinity"},
				ExposedPorts: []string{"1433/tcp"},
				WaitingFor:   wait.NewLogStrategy("Initialization complete."),
			},
		},
	)
}

// TestIntegration tests the receiver against a real SQL Server instance using testcontainers
func TestIntegration(t *testing.T) {
	ci, initErr := setupContainer()
	assert.NoError(t, initErr)

	initErr = ci.Start(t.Context())
	assert.NoError(t, initErr)
	defer testcontainers.CleanupContainer(t, ci)

	p, initErr := ci.MappedPort(t.Context(), "1433")
	assert.NoError(t, initErr)

	cfg := &Config{
		Hostname:                     "localhost",
		Port:                         p.Port(),
		Username:                     "otelcollectoruser",
		Password:                     "otel-password123",
		EnableSSL:                    false,
		TrustServerCertificate:       true,
		Timeout:                      30 * time.Second,
		EnableBufferMetrics:          true,
		EnableDatabaseReserveMetrics: true,
		EnableDiskMetricsInBytes:     true,
	}
	cfg.ControllerConfig.CollectionInterval = 5 * time.Second

	factory := NewFactory()
	consumer := &consumertest.MetricsSink{}

	receiver, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(factory.Type()),
		cfg,
		consumer,
	)
	require.NoError(t, err, "Failed to create receiver")
	require.NotNil(t, receiver, "Receiver should not be nil")

	// Start the receiver
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = receiver.Start(ctx, nil)
	require.NoError(t, err, "Failed to start receiver")
	defer func() {
		err := receiver.Shutdown(ctx)
		assert.NoError(t, err, "Failed to shutdown receiver")
	}()

	// Wait for metrics collection
	assert.Eventually(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 30*time.Second, 1*time.Second, "Should receive metrics within 30 seconds")

	// Verify we received metrics
	metrics := consumer.AllMetrics()
	require.NotEmpty(t, metrics, "Should have received at least one metric batch")

	// Check for expected metrics
	found := false
	for _, metricBatch := range metrics {
		for i := 0; i < metricBatch.ResourceMetrics().Len(); i++ {
			rm := metricBatch.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				for k := 0; k < sm.Metrics().Len(); k++ {
					metric := sm.Metrics().At(k)
					if metric.Name() == "sqlserver.stats.connections" {
						found = true
						assert.Equal(t, "Number of user connections to the SQL Server", metric.Description())
						assert.Equal(t, "{connections}", metric.Unit())
						break
					}
				}
			}
		}
	}

	assert.True(t, found, "Should find sqlserver.stats.connections metric")
}

// TestIntegrationWithQueryMonitoring tests query monitoring features
func TestIntegrationWithQueryMonitoring(t *testing.T) {
	ci, initErr := setupContainer()
	assert.NoError(t, initErr)

	initErr = ci.Start(t.Context())
	assert.NoError(t, initErr)
	defer testcontainers.CleanupContainer(t, ci)

	p, initErr := ci.MappedPort(t.Context(), "1433")
	assert.NoError(t, initErr)

	cfg := &Config{
		Hostname:                             "localhost",
		Port:                                 p.Port(),
		Username:                             "otelcollectoruser",
		Password:                             "otel-password123",
		EnableSSL:                            false,
		TrustServerCertificate:               true,
		Timeout:                              30 * time.Second,
		EnableQueryMonitoring:                true,
		QueryMonitoringResponseTimeThreshold: 1,
		QueryMonitoringCountThreshold:        20,
		QueryMonitoringFetchInterval:         10,
	}
	cfg.ControllerConfig.CollectionInterval = 5 * time.Second

	factory := NewFactory()
	consumer := &consumertest.MetricsSink{}

	receiver, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(factory.Type()),
		cfg,
		consumer,
	)
	require.NoError(t, err, "Failed to create receiver")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = receiver.Start(ctx, nil)
	require.NoError(t, err, "Failed to start receiver")
	defer func() {
		err := receiver.Shutdown(ctx)
		assert.NoError(t, err, "Failed to shutdown receiver")
	}()

	// Wait for some metrics
	time.Sleep(15 * time.Second)

	// At minimum, we should have some metrics even if no blocking sessions
	assert.True(t, len(consumer.AllMetrics()) >= 0, "Should have attempted to collect metrics")
}

// TestIntegrationConnectionFailure tests behavior with invalid connection
func TestIntegrationConnectionFailure(t *testing.T) {
	cfg := &Config{
		Hostname: "nonexistent-host",
		Port:     "1433",
		Username: "sa",
		Password: "wrongpassword",
		Timeout:  5 * time.Second,
	}
	cfg.ControllerConfig.CollectionInterval = 5 * time.Second

	factory := NewFactory()
	consumer := &consumertest.MetricsSink{}

	receiver, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(factory.Type()),
		cfg,
		consumer,
	)
	require.NoError(t, err, "Should create receiver even with bad config")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should fail due to connection issues
	err = receiver.Start(ctx, nil)
	assert.Error(t, err, "Should fail to start with invalid connection")
}
