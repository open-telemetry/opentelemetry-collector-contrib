// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package jmxreceiver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

var jmxJarReleases = map[string]string{
	"1.0.0-alpha": "https://repo1.maven.org/maven2/io/opentelemetry/contrib/opentelemetry-java-contrib-jmx-metrics/1.0.0-alpha/opentelemetry-java-contrib-jmx-metrics-1.0.0-alpha.jar",
	"1.4.0-alpha": "https://repo1.maven.org/maven2/io/opentelemetry/contrib/opentelemetry-jmx-metrics/1.4.0-alpha/opentelemetry-jmx-metrics-1.4.0-alpha.jar",
}

type JMXIntegrationSuite struct {
	suite.Suite
	VersionToJar map[string]string
}

func TestJMXIntegration(t *testing.T) {
	suite.Run(t, new(JMXIntegrationSuite))
}

func (suite *JMXIntegrationSuite) SetupSuite() {
	suite.VersionToJar = make(map[string]string)
	for version, url := range jmxJarReleases {
		jarPath, err := downloadJMXMetricGathererJAR(url)
		require.NoError(suite.T(), err)
		suite.VersionToJar[version] = jarPath
	}
}

func (suite *JMXIntegrationSuite) TearDownSuite() {
	for _, path := range suite.VersionToJar {
		require.NoError(suite.T(), os.Remove(path))
	}
}

func downloadJMXMetricGathererJAR(url string) (string, error) {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	file, err := os.CreateTemp("", "jmx-metrics.jar")
	if err != nil {
		return "", err
	}

	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return file.Name(), err
}

func cassandraContainer(t *testing.T) testcontainers.Container {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "testdata",
			Dockerfile: "Dockerfile.cassandra",
		},
		ExposedPorts: []string{"7199:7199"},
		WaitingFor:   wait.ForListeningPort("7199"),
	}
	cassandra, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	return cassandra
}

func getJavaStdout(receiver *jmxMetricReceiver) string {
	msg := ""
LOOP:
	for i := 0; i < 70; i++ {
		t := time.NewTimer(5 * time.Second)
		select {
		case m, ok := <-receiver.subprocess.Stdout:
			if ok {
				msg = msg + m + "\n"
			} else {
				break LOOP
			}
		case <-t.C:
			break LOOP
		}
	}
	return fmt.Sprintf("metrics not collected: %v\n", msg)
}

func getLogsOnFailure(t *testing.T, logObserver *observer.ObservedLogs) {
	if !t.Failed() {
		return
	}
	fmt.Printf("Logs: \n")
	for _, statement := range logObserver.All() {
		fmt.Printf("%v\n", statement)
	}
}

// Workaround to avoid unused errors
var skip = func(t *testing.T, why string) {
	t.Skip(why)
}

func (suite *JMXIntegrationSuite) TestJMXReceiverHappyPath() {

	for version, jar := range suite.VersionToJar {
		t := suite.T()
		// Run one test per JMX receiver version we're integrating with.
		t.Run(version, func(t *testing.T) {
			skip(t, "https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/5874")

			cassandra := cassandraContainer(t)
			defer func() {
				require.NoError(t, cassandra.Terminate(context.Background()))
			}()
			hostname, err := cassandra.Host(context.Background())
			require.NoError(t, err)

			logCore, logObserver := observer.New(zap.DebugLevel)
			defer getLogsOnFailure(t, logObserver)

			logger := zap.New(logCore)
			params := receivertest.NewNopCreateSettings()
			params.Logger = logger

			cfg := &Config{
				CollectionInterval: 100 * time.Millisecond,
				Endpoint:           fmt.Sprintf("%v:7199", hostname),
				JARPath:            jar,
				TargetSystem:       "cassandra",
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "127.0.0.1:0",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 1000 * time.Millisecond,
					},
				},
				Password: "cassandra",
				Username: "cassandra",
				ResourceAttributes: map[string]string{
					"myattr":      "myvalue",
					"myotherattr": "myothervalue",
				},
				LogLevel: "debug",
			}
			require.NoError(t, cfg.Validate())

			consumer := new(consumertest.MetricsSink)
			require.NotNil(t, consumer)

			receiver := newJMXMetricReceiver(params, cfg, consumer)
			require.NotNil(t, receiver)
			defer func() {
				require.Nil(t, receiver.Shutdown(context.Background()))
			}()

			require.NoError(t, receiver.Start(context.Background(), componenttest.NewNopHost()))

			require.Eventually(t, func() bool {
				found := consumer.DataPointCount() > 0
				if !found {
					return false
				}

				metric := consumer.AllMetrics()[0]
				require.Equal(t, 1, metric.DataPointCount())

				rm := metric.ResourceMetrics().At(0)
				resource := rm.Resource()
				attributes := resource.Attributes()
				lang, ok := attributes.Get("telemetry.sdk.language")
				require.True(t, ok)
				require.Equal(t, "java", lang.Str())

				sdkName, ok := attributes.Get("telemetry.sdk.name")
				require.True(t, ok)
				require.Equal(t, "opentelemetry", sdkName.Str())

				version, ok := attributes.Get("telemetry.sdk.version")
				require.True(t, ok)
				require.NotEmpty(t, version.Str())

				customAttr, ok := attributes.Get("myattr")
				require.True(t, ok)
				require.Equal(t, "myvalue", customAttr.Str())

				anotherCustomAttr, ok := attributes.Get("myotherattr")
				require.True(t, ok)
				require.Equal(t, "myothervalue", anotherCustomAttr.Str())

				ilm := rm.ScopeMetrics().At(0)
				require.Equal(t, "io.opentelemetry.contrib.jmxmetrics", ilm.Scope().Name())
				require.Equal(t, "1.0.0-alpha", ilm.Scope().Version())

				met := ilm.Metrics().At(0)

				require.Equal(t, "cassandra.storage.load", met.Name())
				require.Equal(t, "Size, in bytes, of the on disk data size this node manages", met.Description())
				require.Equal(t, "By", met.Unit())

				// otel-java only uses int sum w/ non-monotonic for up down counters instead of gauge
				require.Equal(t, pmetric.MetricTypeSum, met.Type())
				sum := met.Sum()
				require.False(t, sum.IsMonotonic())

				return true
			}, 30*time.Second, 100*time.Millisecond, getJavaStdout(receiver))
		})
	}
}

func TestJMXReceiverInvalidOTLPEndpointIntegration(t *testing.T) {
	params := receivertest.NewNopCreateSettings()
	cfg := &Config{
		CollectionInterval: 100 * time.Millisecond,
		Endpoint:           "service:jmx:rmi:///jndi/rmi://localhost:7199/jmxrmi",
		JARPath:            "/notavalidpath",
		TargetSystem:       "jvm",
		OTLPExporterConfig: otlpExporterConfig{
			Endpoint: "<invalid>:123",
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 1000 * time.Millisecond,
			},
		},
	}
	receiver := newJMXMetricReceiver(params, cfg, consumertest.NewNop())
	require.NotNil(t, receiver)
	defer func() {
		require.EqualError(t, receiver.Shutdown(context.Background()), "no subprocess.cancel().  Has it been started properly?")
	}()

	err := receiver.Start(context.Background(), componenttest.NewNopHost())
	require.Contains(t, err.Error(), "listen tcp: lookup <invalid>:")
}
