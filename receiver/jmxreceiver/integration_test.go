// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package jmxreceiver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

type integrationConfig struct {
	downloadURL string
	jmxConfig   string
}

var jmxJarReleases = map[string]integrationConfig{
	"1.26.0-alpha": {
		downloadURL: "https://repo1.maven.org/maven2/io/opentelemetry/contrib/opentelemetry-jmx-metrics/1.26.0-alpha/opentelemetry-jmx-metrics-1.26.0-alpha.jar",
	},
	"1.10.0-alpha": {
		downloadURL: "https://repo1.maven.org/maven2/io/opentelemetry/contrib/opentelemetry-jmx-metrics/1.10.0-alpha/opentelemetry-jmx-metrics-1.10.0-alpha.jar",
	},
	"1.46.0-alpha-scraper": {
		downloadURL: "https://repo1.maven.org/maven2/io/opentelemetry/contrib/opentelemetry-jmx-scraper/1.46.0-alpha/opentelemetry-jmx-scraper-1.46.0-alpha.jar",
	},
	"1.46.0-alpha-scraper-custom-jmxconfig": {
		downloadURL: "https://repo1.maven.org/maven2/io/opentelemetry/contrib/opentelemetry-jmx-scraper/1.46.0-alpha/opentelemetry-jmx-scraper-1.46.0-alpha.jar",
		jmxConfig:   filepath.Join("testdata", "integration", "1.46.0-alpha-scraper-custom-jmxconfig", "simple-cassandra.yaml"),
	},
}

// It is recommended that this test be run locally with a longer timeout than the default 30s
// go test -timeout 60s -run ^TestJMXIntegration$ github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver
func TestJMXIntegration(t *testing.T) {
	versionToJar := setupJARs(t)
	t.Cleanup(func() {
		cleanupJARs(t, versionToJar)
	})

	for version, jar := range versionToJar {
		t.Run(version, integrationTest(version, jar, jmxJarReleases[version].jmxConfig))
	}
}

func setupJARs(t *testing.T) map[string]string {
	versionToJar := make(map[string]string)
	for version, config := range jmxJarReleases {
		jarPath, err := downloadJMXJAR(t, config.downloadURL)
		require.NoError(t, err)
		versionToJar[version] = jarPath
	}
	return versionToJar
}

func cleanupJARs(t *testing.T, versionToJar map[string]string) {
	for _, path := range versionToJar {
		require.NoError(t, os.Remove(path))
	}
}

func downloadJMXJAR(t *testing.T, url string) (string, error) {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	file, err := os.CreateTemp(t.TempDir(), "jmx-metrics.jar")
	if err != nil {
		return "", err
	}

	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return file.Name(), err
}

func integrationTest(version, jar, jmxConfig string) func(*testing.T) {
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
				if jmxConfig != "" {
					rCfg.JmxConfigs = jmxConfig
				} else {
					rCfg.TargetSystem = "cassandra"
				}
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
			pmetrictest.IgnoreScopeMetricsOrder(),
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
		require.EqualError(t, receiver.Shutdown(t.Context()), "no subprocess.cancel().  Has it been started properly?")
	}()

	err := receiver.Start(t.Context(), componenttest.NewNopHost())
	require.ErrorContains(t, err, "listen tcp: lookup <invalid>:")
}
