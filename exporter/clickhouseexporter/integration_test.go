// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/goleak"
)

func randPort() int {
	return rand.IntN(999) + 9000
}

func getContainer(req testcontainers.ContainerRequest) (testcontainers.Container, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate container request: %w", err)
	}

	container, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to configure container: %w", err)
	}

	err = container.Start(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	return container, nil
}

func createClickhouseContainer(image string) (testcontainers.Container, string, string, error) {
	port := randPort()
	httpPort := port + 1
	req := testcontainers.ContainerRequest{
		Image: image,
		ExposedPorts: []string{
			fmt.Sprintf("%d:9000", port),
			fmt.Sprintf("%d:8123", httpPort),
		},
		WaitingFor: wait.ForListeningPort("9000").
			WithStartupTimeout(2 * time.Minute),
		Env: map[string]string{
			"CLICKHOUSE_PASSWORD": "otel",
		},
	}

	c, err := getContainer(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("getContainer: %w", err)
	}

	host, err := c.Host(context.Background())
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to read container host address: %w", err)
	}

	nativeEndpoint := fmt.Sprintf("tcp://%s:%d?username=default&password=otel&database=otel_int_test", host, port)
	httpEndpoint := fmt.Sprintf("http://%s:%d?username=default&password=otel&database=otel_int_test", host, httpPort)

	return c, nativeEndpoint, httpEndpoint, nil
}

func withTestExporterConfig(fns ...func(*Config)) func(string) *Config {
	return func(endpoint string) *Config {
		var configMods []func(*Config)
		configMods = append(configMods, func(cfg *Config) {
			cfg.Endpoint = endpoint
		})
		configMods = append(configMods, fns...)
		return withDefaultConfig(configMods...)
	}
}

var telemetryTimestamp = time.Unix(1703498029, 0).UTC()

func TestIntegration(t *testing.T) {
	c, nativeEndpoint, httpEndpoint, err := createClickhouseContainer("clickhouse/clickhouse-server:25.5-alpine")
	if err != nil {
		panic(fmt.Errorf("failed to create ClickHouse container: %w", err))
	}
	defer func(c testcontainers.Container) {
		err := c.Terminate(context.Background())
		if err != nil {
			fmt.Println(fmt.Errorf("failed to terminate ClickHouse container: %w", err))
		}
	}(c)

	t.Run("TestLogsExporter", func(t *testing.T) {
		t.Run("Native", func(t *testing.T) {
			testLogsExporter(t, nativeEndpoint)
		})
		t.Run("HTTP", func(t *testing.T) {
			testLogsExporter(t, httpEndpoint)
		})
	})
	t.Run("TestTracesExporter", func(t *testing.T) {
		t.Run("Native", func(t *testing.T) {
			testTracesExporter(t, nativeEndpoint)
		})
		t.Run("HTTP", func(t *testing.T) {
			testTracesExporter(t, httpEndpoint)
		})
	})
	t.Run("TestMetricsExporter", func(t *testing.T) {
		t.Run("Native", func(t *testing.T) {
			testMetricsExporter(t, nativeEndpoint)
		})
		t.Run("HTTP", func(t *testing.T) {
			testMetricsExporter(t, httpEndpoint)
		})
	})

	// Verify all integration tests, ignoring test container reaper
	goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/testcontainers/testcontainers-go.(*Reaper).connect.func1"))
}
