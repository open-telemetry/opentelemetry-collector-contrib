// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package jmxreceiver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver/internal/metadata"
)

const jmxPort = "7199"

var jmxJarReleases = map[string]string{
	"1.26.0-alpha": "https://repo1.maven.org/maven2/io/opentelemetry/contrib/opentelemetry-jmx-metrics/1.26.0-alpha/opentelemetry-jmx-metrics-1.26.0-alpha.jar",
	"1.10.0-alpha": "https://repo1.maven.org/maven2/io/opentelemetry/contrib/opentelemetry-jmx-metrics/1.10.0-alpha/opentelemetry-jmx-metrics-1.10.0-alpha.jar",
}

type jmxIntegrationSuite struct {
	suite.Suite
	VersionToJar map[string]string
}

// It is recommended that this test be run locally with a longer timeout than the default 30s
// go test -timeout 60s -run ^TestJMXIntegration$ github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver
func TestJMXIntegration(t *testing.T) {
	suite.Run(t, new(jmxIntegrationSuite))
}

func (suite *jmxIntegrationSuite) SetupSuite() {
	suite.VersionToJar = make(map[string]string)
	for version, url := range jmxJarReleases {
		jarPath, err := downloadJMXMetricGathererJAR(url)
		suite.VersionToJar[version] = jarPath
		suite.Require().NoError(err)
	}
}

func (suite *jmxIntegrationSuite) TearDownSuite() {
	for _, path := range suite.VersionToJar {
		suite.Require().NoError(os.Remove(path))
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

func (suite *jmxIntegrationSuite) TestJMXReceiverHappyPath() {
	for version, jar := range suite.VersionToJar {
		suite.T().Run(version, integrationTest(version, jar))
	}
}

func integrationTest(version string, jar string) func(*testing.T) {
	return scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image: "cassandra:3.11",
				Env: map[string]string{
					"LOCAL_JMX": "no",
					"JVM_OPTS":  "-Djava.rmi.server.hostname=0.0.0.0",
				},
				Files: []testcontainers.ContainerFile{{
					HostFilePath:      filepath.Join("testdata", "integration", "jmxremote.password"),
					ContainerFilePath: "/etc/cassandra/jmxremote.password",
					FileMode:          400,
				}},
				ExposedPorts: []string{jmxPort + ":" + jmxPort},
				WaitingFor:   wait.ForListeningPort(jmxPort),
			}),
		scraperinttest.AllowHardcodedHostPort(),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = 3 * time.Second
				rCfg.JARPath = jar
				rCfg.Endpoint = fmt.Sprintf("%v:%s", ci.Host(t), ci.MappedPort(t, jmxPort))
				rCfg.TargetSystem = "cassandra"
				rCfg.Username = "cassandra"
				rCfg.Password = "cassandra"
				rCfg.ResourceAttributes = map[string]string{
					"myattr":      "myvalue",
					"myotherattr": "myothervalue",
				}
				rCfg.OTLPExporterConfig = otlpExporterConfig{
					Endpoint: "127.0.0.1:0",
					TimeoutSettings: exporterhelper.TimeoutConfig{
						Timeout: time.Second,
					},
				}
			}),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "integration", version, "expected.yaml")),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
		),
	).Run
}

func TestJMXReceiverInvalidOTLPEndpointIntegration(t *testing.T) {
	params := receivertest.NewNopSettings(metadata.Type)
	cfg := &Config{
		CollectionInterval: 100 * time.Millisecond,
		Endpoint:           "service:jmx:rmi:///jndi/rmi://localhost:7199/jmxrmi",
		JARPath:            "/notavalidpath",
		TargetSystem:       "jvm",
		OTLPExporterConfig: otlpExporterConfig{
			Endpoint: "<invalid>:123",
			TimeoutSettings: exporterhelper.TimeoutConfig{
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
	require.ErrorContains(t, err, "listen tcp: lookup <invalid>:")
}
