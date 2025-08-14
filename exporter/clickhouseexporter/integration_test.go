// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

	nativeEndpoint := fmt.Sprintf("tcp://%s:%d?username=default&password=otel&enable_json_type=1&database=otel_int_test", host, port)
	httpEndpoint := fmt.Sprintf("http://%s:%d?username=default&password=otel&enable_json_type=1&database=otel_int_test", host, httpPort)

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

func pushConcurrentlyNoError(t *testing.T, fn func() error) {
	var (
		count = 5
		errs  = make(chan error, count)
		wg    sync.WaitGroup
	)

	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			err := fn()
			errs <- err
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
}

var telemetryTimestamp = time.Unix(1703498029, 0).UTC()

func testIntegrationWithImage(t *testing.T, clickhouseImage string) {
	c, nativeEndpoint, httpEndpoint, err := createClickhouseContainer(clickhouseImage)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to create ClickHouse container \"%s\": %w", clickhouseImage, err))
	}
	defer func(c testcontainers.Container) {
		err := c.Terminate(t.Context())
		if err != nil {
			fmt.Println(fmt.Errorf("failed to terminate ClickHouse container \"%s\": %w", clickhouseImage, err))
		}
	}(c)

	// For regular integration tests that need to be tested with both protocols
	testProtocols := func(exporterTest func(*testing.T, string)) func(t *testing.T) {
		return func(t *testing.T) {
			t.Run("Native", func(t *testing.T) {
				exporterTest(t, nativeEndpoint)
			})
			t.Run("HTTP", func(t *testing.T) {
				exporterTest(t, httpEndpoint)
			})
		}
	}

	// For log tests where the log Body could be a string or a Map
	testProtocolsMapBody := func(exporterTest func(*testing.T, string, bool)) func(t *testing.T) {
		return func(t *testing.T) {
			t.Run("String Body", func(t *testing.T) {
				t.Run("Native", func(t *testing.T) {
					exporterTest(t, nativeEndpoint, false)
				})
				t.Run("HTTP", func(t *testing.T) {
					exporterTest(t, httpEndpoint, false)
				})
			})

			t.Run("Map Body", func(t *testing.T) {
				t.Run("Native", func(t *testing.T) {
					exporterTest(t, nativeEndpoint, true)
				})
				t.Run("HTTP", func(t *testing.T) {
					exporterTest(t, httpEndpoint, true)
				})
			})
		}
	}

	t.Run("TestLogsExporter", testProtocolsMapBody(testLogsExporter))
	t.Run("TestTracesExporter", testProtocols(testTracesExporter))
	t.Run("TestMetricsExporter", testProtocols(testMetricsExporter))
	t.Run("TestLogsJSONExporter", testProtocolsMapBody(testLogsJSONExporter))
	t.Run("TestTracesJSONExporter", testProtocols(testTracesJSONExporter))
}

func TestIntegration(t *testing.T) {
	// Update versions according to oldest and newest supported here: https://github.com/clickhouse/clickhouse/security
	t.Run("25.6", func(t *testing.T) {
		testIntegrationWithImage(t, "clickhouse/clickhouse-server:25.6-alpine")
	})
	t.Run("24.11", func(t *testing.T) {
		testIntegrationWithImage(t, "clickhouse/clickhouse-server:24.11-alpine")
	})

	// Verify all integration tests, ignoring test container reaper
	goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/testcontainers/testcontainers-go.(*Reaper).connect.func1"))
}
