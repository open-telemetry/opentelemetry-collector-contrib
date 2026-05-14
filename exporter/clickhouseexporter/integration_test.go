// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/goleak"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
)

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

type clickhouseEnv struct {
	NativeEndpoint       string
	HTTPEndpoint         string
	SecureNativeEndpoint string
	HTTPSEndpoint        string
}

type stdoutLogConsumer struct{}

func (*stdoutLogConsumer) Accept(l testcontainers.Log) {
	fmt.Print(string(l.Content))
}

func createClickhouseContainer(image string) (testcontainers.Container, *clickhouseEnv, error) {
	_, b, _, _ := runtime.Caller(0)
	basePath := filepath.Dir(b)

	fileMount := func(testDataPath, containerPath string) testcontainers.ContainerFile {
		return testcontainers.ContainerFile{
			HostFilePath:      path.Join(basePath, "testdata", testDataPath),
			ContainerFilePath: containerPath,
			FileMode:          0o644,
		}
	}

	var lc stdoutLogConsumer
	req := testcontainers.ContainerRequest{
		Image: image,
		ExposedPorts: []string{
			"9000/tcp",
			"8123/tcp",
			"9440/tcp",
			"8443/tcp",
		},
		Files: []testcontainers.ContainerFile{
			fileMount("clickhouse-config.xml", "/etc/clickhouse-server/config.d/otel.xml"),
			fileMount("clickhouse-users.xml", "/etc/clickhouse-server/users.d/otel-users.xml"),
			fileMount("certs/CAroot.crt", "/etc/clickhouse-server/certs/CAroot.crt"),
			fileMount("certs/server.crt", "/etc/clickhouse-server/certs/server.crt"),
			fileMount("certs/server.key", "/etc/clickhouse-server/certs/server.key"),
		},
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Opts:      []testcontainers.LogProductionOption{testcontainers.WithLogProductionTimeout(10 * time.Second)},
			Consumers: []testcontainers.LogConsumer{&lc},
		},
		WaitingFor: wait.ForListeningPort("9000").
			WithStartupTimeout(2 * time.Minute),
	}

	c, err := getContainer(req)
	if err != nil {
		return nil, nil, fmt.Errorf("getContainer: %w", err)
	}

	host, err := c.Host(context.Background())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read container host address: %w", err)
	}

	nativePort, err := c.MappedPort(context.Background(), "9000")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get mapped port 9000: %w", err)
	}
	httpPort, err := c.MappedPort(context.Background(), "8123")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get mapped port 8123: %w", err)
	}
	tlsPort, err := c.MappedPort(context.Background(), "9440")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get mapped port 9440: %w", err)
	}
	httpsPort, err := c.MappedPort(context.Background(), "8443")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get mapped port 8443: %w", err)
	}

	env := clickhouseEnv{
		NativeEndpoint:       fmt.Sprintf("tcp://%s:%s?username=default&password=otel&enable_json_type=1&database=otel_int_test", host, nativePort.Port()),
		HTTPEndpoint:         fmt.Sprintf("http://%s:%s?username=default&password=otel&enable_json_type=1&database=otel_int_test", host, httpPort.Port()),
		SecureNativeEndpoint: fmt.Sprintf("tcp://%s:%s?username=secure_default&enable_json_type=1&database=otel_int_test", host, tlsPort.Port()),
		HTTPSEndpoint:        fmt.Sprintf("https://%s:%s?username=secure_default&enable_json_type=1&database=otel_int_test", host, httpsPort.Port()),
	}

	return c, &env, nil
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
	for range count {
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
	c, chEnv, err := createClickhouseContainer(clickhouseImage)
	if err != nil {
		t.Fatalf("failed to create ClickHouse container %q: %v", clickhouseImage, err)
	}
	defer func(c testcontainers.Container) {
		err := c.Terminate(t.Context())
		if err != nil {
			t.Logf("failed to terminate ClickHouse container %q: %v", clickhouseImage, err)
		}
	}(c)

	// For regular integration tests that need to be tested with both protocols
	testProtocols := func(exporterTest func(*testing.T, string), secure bool) func(t *testing.T) {
		nativeEndpoint := chEnv.NativeEndpoint
		httpEndpoint := chEnv.HTTPEndpoint
		if secure {
			nativeEndpoint = chEnv.SecureNativeEndpoint
			httpEndpoint = chEnv.HTTPSEndpoint
		}

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
					exporterTest(t, chEnv.NativeEndpoint, false)
				})
				t.Run("HTTP", func(t *testing.T) {
					exporterTest(t, chEnv.HTTPEndpoint, false)
				})
			})

			t.Run("Map Body", func(t *testing.T) {
				t.Run("Native", func(t *testing.T) {
					exporterTest(t, chEnv.NativeEndpoint, true)
				})
				t.Run("HTTP", func(t *testing.T) {
					exporterTest(t, chEnv.HTTPEndpoint, true)
				})
			})
		}
	}

	t.Run("TestLogsExporter", testProtocolsMapBody(testLogsExporter))
	t.Run("TestLogsExporterSchemaFeatures", testProtocolsMapBody(testLogsExporterSchemaFeatures))
	t.Run("TestTracesExporter", testProtocols(testTracesExporter, false))
	t.Run("TestMetricsExporter", testProtocols(testMetricsExporter, false))
	t.Run("TestLogsJSONExporter", testProtocolsMapBody(testLogsJSONExporter))
	t.Run("TestLogsJSONExporterSchemaFeatures", testProtocolsMapBody(testLogsJSONExporterSchemaFeatures))
	t.Run("TestTracesJSONExporter", testProtocols(testTracesJSONExporter, false))
	t.Run("TestTracesJSONExporterSchemaFeatures", testProtocols(testTracesJSONExporterSchemaFeatures, false))
	t.Run("TestLogsCombinedExporter", testProtocolsMapBody(testLogsCombinedExporter))
	t.Run("TestTracesCombinedExporter", testProtocols(testTracesCombinedExporter, false))

	t.Run("TestCertAuth", testProtocols(func(t *testing.T, dsn string) {
		applyTLS := func(config *Config) {
			config.TLS.CAFile = "./testdata/certs/CAroot.crt"
			config.TLS.CertFile = "./testdata/certs/client.crt"
			config.TLS.KeyFile = "./testdata/certs/client.key"
		}
		cfg := withTestExporterConfig(applyTLS)(dsn)
		opt, err := cfg.buildClickHouseOptions()
		if err != nil {
			t.Fatal(err)
		}

		db, err := internal.NewClickhouseClientFromOptions(opt)
		if err != nil {
			t.Fatal(err)
		}

		err = db.Ping(t.Context())
		if err != nil {
			t.Fatal(err)
		}

		err = db.Close()
		if err != nil {
			t.Fatal(err)
		}
	}, true))
}

func TestIntegration(t *testing.T) {
	// Update versions according to oldest and newest supported here: https://github.com/clickhouse/clickhouse/security
	// 25.8 tests the bloom_filter fallback (CH < 26.2), 26.2 tests full text search indexes.
	t.Run("26.2", func(t *testing.T) {
		testIntegrationWithImage(t, "clickhouse/clickhouse-server:26.2-alpine")
	})
	t.Run("25.8", func(t *testing.T) {
		testIntegrationWithImage(t, "clickhouse/clickhouse-server:25.8-alpine")
	})

	// Verify all integration tests, ignoring test container reaper
	goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/testcontainers/testcontainers-go.(*Reaper).connect.func1"))
}
