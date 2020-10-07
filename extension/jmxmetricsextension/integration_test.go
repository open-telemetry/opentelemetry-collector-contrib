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

// +build integration

package jmxmetricsextension

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	prommodel "github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	promdiscovery "github.com/prometheus/prometheus/discovery"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/prometheusreceiver"
	"go.opentelemetry.io/collector/testbed/testbed"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type JmxIntegrationSuite struct {
	suite.Suite
	JarPath string
}

func TestJmxIntegration(t *testing.T) {
	suite.Run(t, new(JmxIntegrationSuite))
}

func (suite *JmxIntegrationSuite) SetupSuite() {
	jarPath, err := downloadJmxMetricGathererJar()
	require.NoError(suite.T(), err)
	suite.JarPath = jarPath
}

func (suite *JmxIntegrationSuite) TearDownSuite() {
	require.NoError(suite.T(), os.Remove(suite.JarPath))
}

func downloadJmxMetricGathererJar() (string, error) {
	url := "https://oss.jfrog.org/artifactory/list/oss-snapshot-local/io/opentelemetry/contrib/opentelemetry-java-contrib-jmx-metrics/0.0.1-SNAPSHOT/opentelemetry-java-contrib-jmx-metrics-0.0.1-20200918.184353-3.jar"
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

func newOtlpMetricReceiver(t *testing.T, logger *zap.Logger) (int, *component.MetricsReceiver, *exportertest.SinkMetricsExporter) {
	consumer := &exportertest.SinkMetricsExporter{}
	require.NotNil(t, consumer)

	port := testbed.GetAvailablePort(t)

	factory := otlpreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*otlpreceiver.Config)
	cfg.SetName("otlp")

	cfg.GRPC.NetAddr = confignet.NetAddr{Endpoint: fmt.Sprintf("0.0.0.0:%d", port), Transport: "tcp"}
	cfg.HTTP = nil
	params := component.ReceiverCreateParams{Logger: logger}

	var err error
	var receiver component.MetricsReceiver
	if receiver, err = factory.CreateMetricsReceiver(context.Background(), params, cfg, consumer); err == nil {
		err = receiver.Start(context.Background(), componenttest.NewNopHost())
	}
	require.NotNil(t, receiver)
	require.NoError(t, err)

	return port, &receiver, consumer
}

func newPrometheusMetricReceiver(t *testing.T, logger *zap.Logger) (int, *component.MetricsReceiver, *exportertest.SinkMetricsExporter) {
	consumer := &exportertest.SinkMetricsExporter{}
	require.NotNil(t, consumer)

	port := testbed.GetAvailablePort(t)

	factory := prometheusreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*prometheusreceiver.Config)
	cfg.SetName("prometheus")

	cfg.PrometheusConfig = &promconfig.Config{
		ScrapeConfigs: []*promconfig.ScrapeConfig{
			{
				JobName:         "jmx_metrics",
				ScrapeInterval:  prommodel.Duration(100 * time.Millisecond),
				ScrapeTimeout:   prommodel.Duration(1 * time.Second),
				Scheme:          "http",
				MetricsPath:     "/metrics",
				HonorTimestamps: true,
				ServiceDiscoveryConfigs: promdiscovery.Configs{
					&promdiscovery.StaticConfig{
						{
							Targets: []prommodel.LabelSet{
								{prommodel.AddressLabel: prommodel.LabelValue(fmt.Sprintf("localhost:%v", port))},
							},
							Labels: prommodel.LabelSet{
								"myPrometheusLabel": "myPrometheusLabelValue",
							},
						},
					},
				},
			},
		},
	}
	params := component.ReceiverCreateParams{Logger: logger}

	var err error
	var receiver component.MetricsReceiver
	if receiver, err = factory.CreateMetricsReceiver(context.Background(), params, cfg, consumer); err == nil {
		err = receiver.Start(context.Background(), componenttest.NewNopHost())
	}
	require.NotNil(t, receiver)
	require.NoError(t, err)

	return port, &receiver, consumer
}

func getJavaStdout(extension *jmxMetricsExtension) string {
	msg := ""
LOOP:
	for i := 0; i < 70; i++ {
		t := time.NewTimer(5 * time.Second)
		select {
		case m, ok := <-extension.subprocess.Stdout:
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

func (suite *JmxIntegrationSuite) TestJmxMetricViaOtlpReceiverIntegration() {
	t := suite.T()
	cassandra := cassandraContainer(t)
	defer cassandra.Terminate(context.Background())
	hostname, err := cassandra.Host(context.Background())
	require.NoError(t, err)

	logCore, logObserver := observer.New(zap.DebugLevel)
	defer getLogsOnFailure(t, logObserver)

	logger := zap.New(logCore)
	port, receiver, consumer := newOtlpMetricReceiver(t, logger)
	defer func() {
		require.Nil(t, (*receiver).Shutdown(context.Background()))
	}()

	otlp := "localhost"
	if runtime.GOOS == "darwin" {
		otlp = "host.docker.internal"
	}

	config := &config{
		JarPath:      suite.JarPath,
		ServiceURL:   fmt.Sprintf("service:jmx:rmi:///jndi/rmi://%v:7199/jmxrmi", hostname),
		Exporter:     "otlp",
		OtlpEndpoint: fmt.Sprintf("%v:%v", otlp, port),
		GroovyScript: path.Join(".", "testdata", "script.groovy"),
		Username:     "cassandra",
		Password:     "cassandra",
	}

	extension := newJmxMetricsExtension(logger, config)
	require.NotNil(t, extension)
	defer func() {
		require.Nil(t, extension.Shutdown(context.Background()))
	}()

	require.NoError(t, extension.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, extension.Ready())

	require.Eventually(t, func() bool {
		found := consumer.MetricsCount() == 1
		if !found {
			return false
		}

		metric := consumer.AllMetrics()[0]
		metricCount, datapointCount := metric.MetricAndDataPointCount()
		require.Equal(t, 1, metricCount)
		require.Equal(t, 1, datapointCount)

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

		ilm := rm.InstrumentationLibraryMetrics().At(0)
		require.Equal(t, "io.opentelemetry.contrib.jmxmetrics", ilm.InstrumentationLibrary().Name())
		require.Equal(t, "0.0.1", ilm.InstrumentationLibrary().Version())

		met := ilm.Metrics().At(0)

		require.Equal(t, "cassandra.storage.load", met.Name())
		require.Equal(t, "Size, in bytes, of the on disk data size this node manages", met.Description())
		require.Equal(t, "By", met.Unit())

		// otel-java only uses int sum w/ non-monotonic for up down counters instead of gauge
		require.Equal(t, pdata.MetricDataTypeIntSum, met.DataType())
		sum := met.IntSum()
		require.False(t, sum.IsMonotonic())

		return true
	}, 30*time.Second, 100*time.Millisecond, getJavaStdout(extension))
}

func (suite *JmxIntegrationSuite) TestJmxMetricViaPrometheusReceiverIntegration() {
	t := suite.T()
	cassandra := cassandraContainer(t)
	defer cassandra.Terminate(context.Background())
	hostname, err := cassandra.Host(context.Background())
	require.NoError(t, err)

	logCore, logObserver := observer.New(zap.DebugLevel)
	defer getLogsOnFailure(t, logObserver)

	logger := zap.New(logCore)
	port, receiver, consumer := newPrometheusMetricReceiver(t, logger)
	defer func() {
		require.Nil(t, (*receiver).Shutdown(context.Background()))
	}()

	config := &config{
		JarPath:       suite.JarPath,
		ServiceURL:    fmt.Sprintf("service:jmx:rmi:///jndi/rmi://%v:7199/jmxrmi", hostname),
		Exporter:      "prometheus",
		PromethusHost: "localhost",
		PromethusPort: port,
		Interval:      100 * time.Millisecond,
		GroovyScript:  path.Join(".", "testdata", "script.groovy"),
		Username:      "cassandra",
		Password:      "cassandra",
	}

	extension := newJmxMetricsExtension(logger, config)
	require.NotNil(t, extension)
	defer func() {
		require.Nil(t, extension.Shutdown(context.Background()))
	}()

	require.NoError(t, extension.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, extension.Ready())

	require.Eventually(t, func() bool {
		metrics := consumer.AllMetrics()
		if len(metrics) == 0 {
			return false
		}
		for _, metric := range metrics {
			rms := metric.ResourceMetrics()
			rmsLen := rms.Len()
			require.Equal(t, 1, rmsLen)
			rm := rms.At(0)
			ilms := rm.InstrumentationLibraryMetrics()
			ilmsLen := ilms.Len()
			require.Equal(t, 1, ilmsLen)
			ilm := rm.InstrumentationLibraryMetrics().At(0)
			mets := ilm.Metrics()
			metsLen := mets.Len()
			require.Equal(t, 1, metsLen)
			met := mets.At(0)
			require.False(t, met.IsNil())
			require.Equal(t, "cassandra_storage_load", met.Name())
			require.Equal(t, "Size, in bytes, of the on disk data size this node manages", met.Description())
			require.Equal(t, pdata.MetricDataTypeDoubleGauge, met.DataType())
			gauge := met.DoubleGauge()
			dps := gauge.DataPoints()
			require.Equal(t, 1, dps.Len())
			dp := dps.At(0)
			require.Greater(t, dp.Value(), 0.0)
		}
		return true
	}, 30*time.Second, 10*time.Millisecond, getJavaStdout(extension))
}
