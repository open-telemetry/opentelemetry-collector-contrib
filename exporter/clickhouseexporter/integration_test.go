// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"context"
	"fmt"
	"go.uber.org/goleak"
	"math/rand/v2"
	"strconv"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func randPort() string {
	return strconv.Itoa(rand.IntN(999) + 9000)
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

func createClickhouseContainer(image string) (testcontainers.Container, string, error) {
	port := randPort()
	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{fmt.Sprintf("%s:9000", port)},
		WaitingFor: wait.ForListeningPort("9000").
			WithStartupTimeout(2 * time.Minute),
		Env: map[string]string{
			"CLICKHOUSE_PASSWORD": "otel",
		},
	}

	c, err := getContainer(req)
	if err != nil {
		return nil, "", fmt.Errorf("getContainer: %w", err)
	}

	host, err := c.Host(context.Background())
	if err != nil {
		return nil, "", fmt.Errorf("failed to read container host address: %w", err)
	}

	endpoint := fmt.Sprintf("tcp://%s:%s?username=default&password=otel&database=otel_int_test", host, port)

	return c, endpoint, nil
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

var integrationTestEndpoint string

var telemetryTimestamp = time.Unix(1703498029, 0).UTC()

func TestIntegration(t *testing.T) {
	c, endpoint, err := createClickhouseContainer("clickhouse/clickhouse-server:25.5-alpine")
	if err != nil {
		panic(fmt.Errorf("failed to create ClickHouse container: %w", err))
	}
	defer func(c testcontainers.Container) {
		err := c.Terminate(context.Background())
		if err != nil {
			fmt.Println(fmt.Errorf("failed to terminate ClickHouse container: %w", err))
		}
	}(c)

	integrationTestEndpoint = endpoint

	t.Run("TestLogsExporter", testLogsExporter)
	t.Run("TestTracesExporter", testTracesExporter)
	t.Run("TestMetricsExporter", testMetricsExporter)

	// Verify all integration tests, ignoring test container reaper
	goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/testcontainers/testcontainers-go.(*Reaper).connect.func1"))
}
