// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	integrationtestutils "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter/internal/integrationtestutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

type SplunkContainerConfig struct {
	conCtx    context.Context
	container testcontainers.Container
}

func setup() SplunkContainerConfig {
	// Perform setup operations here
	cfg := startSplunk()
	return cfg
}

func teardown(cfg SplunkContainerConfig) {
	// Perform teardown operations here
	fmt.Println("Tearing down...")
	// Stop and remove the container
	fmt.Println("Stopping container")
	err := cfg.container.Terminate(cfg.conCtx)
	if err != nil {
		fmt.Printf("Error while terminating container")
		panic(err)
	}
	// Remove docker image after tests
	splunkImage := integrationtestutils.GetConfigVariable("SPLUNK_IMAGE")
	cmd := exec.Command("docker", "rmi", splunkImage)

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error removing Docker image: %v\n", err)
	}
	fmt.Printf("Removed Docker image: %s\n", splunkImage)
	fmt.Printf("Command output:\n%s\n", output)
}

func createInsecureClient() *http.Client {
	// Create a custom transport with insecure settings
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	// Create the client with the custom transport
	client := &http.Client{
		Transport: tr,
	}

	return client
}

func startSplunk() SplunkContainerConfig {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	conContext := context.Background()

	// Create a new container
	splunkImage := integrationtestutils.GetConfigVariable("SPLUNK_IMAGE")
	req := testcontainers.ContainerRequest{
		Image:        splunkImage,
		ExposedPorts: []string{"8000/tcp", "8088/tcp", "8089/tcp"},
		Env: map[string]string{
			"SPLUNK_START_ARGS": "--accept-license",
			"SPLUNK_HEC_TOKEN":  integrationtestutils.GetConfigVariable("HEC_TOKEN"),
			"SPLUNK_PASSWORD":   integrationtestutils.GetConfigVariable("PASSWORD"),
		},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      filepath.Join("testdata", "splunk.yaml"),
				ContainerFilePath: "/tmp/defaults/default.yml",
				FileMode:          0o644,
			},
		},
		WaitingFor: wait.ForHealthCheck().WithStartupTimeout(5 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(conContext, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		logger.Info("Error while creating container")
		panic(err)
	}

	// Get the container host and port
	uiPort, err := container.MappedPort(conContext, "8000")
	if err != nil {
		logger.Info("Error while getting port")
		panic(err)
	}

	hecPort, err := container.MappedPort(conContext, "8088")
	if err != nil {
		logger.Info("Error while getting port")
		panic(err)
	}
	managementPort, err := container.MappedPort(conContext, "8089")
	if err != nil {
		logger.Info("Error while getting port")
		panic(err)
	}
	host, err := container.Host(conContext)
	if err != nil {
		logger.Info("Error while getting host")
		panic(err)
	}

	// Use the container's host and port for your tests
	logger.Info("Splunk running at:", zap.String("host", host), zap.Int("uiPort", uiPort.Int()), zap.Int("hecPort", hecPort.Int()), zap.Int("managementPort", managementPort.Int()))
	integrationtestutils.SetConfigVariable("HOST", host)
	integrationtestutils.SetConfigVariable("UI_PORT", strconv.Itoa(uiPort.Int()))
	integrationtestutils.SetConfigVariable("HEC_PORT", strconv.Itoa(hecPort.Int()))
	integrationtestutils.SetConfigVariable("MANAGEMENT_PORT", strconv.Itoa(managementPort.Int()))
	cfg := SplunkContainerConfig{
		conCtx:    conContext,
		container: container,
	}
	return cfg
}

func prepareLogs() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("test")
	ts := pcommon.Timestamp(0)
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("test log")
	logRecord.Attributes().PutStr(splunk.DefaultNameLabel, "test- label")
	logRecord.Attributes().PutStr(string(conventions.HostNameKey), "myhost")
	logRecord.Attributes().PutStr("custom", "custom")
	logRecord.SetTimestamp(ts)
	return logs
}

func prepareLogsNonDefaultParams(index string, source string, sourcetype string, event string) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("test")
	ts := pcommon.Timestamp(0)

	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr(event)
	logRecord.Attributes().PutStr(splunk.DefaultNameLabel, "label")
	logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, source)
	logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, sourcetype)
	logRecord.Attributes().PutStr(splunk.DefaultIndexLabel, index)
	logRecord.Attributes().PutStr(string(conventions.HostNameKey), "myhost")
	logRecord.Attributes().PutStr("custom", "custom")
	logRecord.SetTimestamp(ts)
	return logs
}

func prepareMetricsData(metricName string) pmetric.Metrics {
	metricData := pmetric.NewMetrics()
	metric := metricData.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	g := metric.SetEmptyGauge()
	g.DataPoints().AppendEmpty().SetDoubleValue(132.929)
	metric.SetName(metricName)
	return metricData
}

func prepareTracesData(index string, source string, sourcetype string) ptrace.Traces {
	ts := pcommon.Timestamp(0)

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("com.splunk.source", source)
	rs.Resource().Attributes().PutStr("host.name", "myhost")
	rs.Resource().Attributes().PutStr("com.splunk.sourcetype", sourcetype)
	rs.Resource().Attributes().PutStr("com.splunk.index", index)
	ils := rs.ScopeSpans().AppendEmpty()
	initSpan("myspan", ts, ils.Spans().AppendEmpty())
	return traces
}

type cfg struct {
	event      string
	index      string
	source     string
	sourcetype string
}

type telemetryType string

var (
	metricsType = telemetryType("metrics")
	logsType    = telemetryType("logs")
	tracesType  = telemetryType("traces")
)

type testCfg struct {
	name      string
	config    *cfg
	startTime string
	telType   telemetryType
}

func logsTest(t *testing.T, config *Config, url *url.URL, test testCfg) {
	settings := exportertest.NewNopSettings(metadata.Type)
	c := newLogsClient(settings, config)
	var logs plog.Logs
	if test.config.index != "main" {
		logs = prepareLogsNonDefaultParams(test.config.index, test.config.source, test.config.sourcetype, test.config.event)
	} else {
		logs = prepareLogs()
	}

	httpClient := createInsecureClient()
	c.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(config, component.NewDefaultBuildInfo()), settings.Logger}

	err := c.pushLogData(context.Background(), logs)
	require.NoError(t, err, "Must not error while sending Logs data")
	waitForEventToBeIndexed()

	events := integrationtestutils.CheckEventsFromSplunk("index="+test.config.index+" *", test.startTime)
	assert.Len(t, events, 1)
	// check events fields
	data, ok := events[0].(map[string]any)
	assert.True(t, ok, "Invalid event format")
	assert.Equal(t, test.config.event, data["_raw"].(string))
	assert.Equal(t, test.config.index, data["index"].(string))
	assert.Equal(t, test.config.source, data["source"].(string))
	assert.Equal(t, test.config.sourcetype, data["sourcetype"].(string))
}

func metricsTest(t *testing.T, config *Config, url *url.URL, test testCfg) {
	settings := exportertest.NewNopSettings(metadata.Type)
	c := newMetricsClient(settings, config)
	metricData := prepareMetricsData(test.config.event)

	httpClient := createInsecureClient()
	c.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(config, component.NewDefaultBuildInfo()), settings.Logger}

	err := c.pushMetricsData(context.Background(), metricData)
	require.NoError(t, err, "Must not error while sending Metrics data")
	waitForEventToBeIndexed()

	events := integrationtestutils.CheckMetricsFromSplunk(test.config.index, test.config.event)
	assert.Len(t, events, 1, "Events length is less than 1. No metrics found")
}

func tracesTest(t *testing.T, config *Config, url *url.URL, test testCfg) {
	settings := exportertest.NewNopSettings(metadata.Type)
	c := newTracesClient(settings, config)
	tracesData := prepareTracesData(test.config.index, test.config.source, test.config.sourcetype)

	httpClient := createInsecureClient()
	c.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(config, component.NewDefaultBuildInfo()), settings.Logger}

	err := c.pushTraceData(context.Background(), tracesData)
	require.NoError(t, err, "Must not error while sending Trace data")
	waitForEventToBeIndexed()

	events := integrationtestutils.CheckEventsFromSplunk("index="+test.config.index+" *", test.startTime)
	assert.Len(t, events, 1)
	// check fields
	data, ok := events[0].(map[string]any)
	assert.True(t, ok, "Invalid event format")
	assert.Equal(t, test.config.index, data["index"].(string))
	assert.Equal(t, test.config.source, data["source"].(string))
	assert.Equal(t, test.config.sourcetype, data["sourcetype"].(string))
}

func TestSplunkHecExporter(t *testing.T) {
	splunkContCfg := setup()
	defer teardown(splunkContCfg)

	tests := []testCfg{
		{
			name: "Events to Splunk",
			config: &cfg{
				event:      "test log",
				index:      "main",
				source:     "otel",
				sourcetype: "st-otel",
			},
			startTime: "-3h@h",
			telType:   logsType,
		},
		{
			name: "Events to Splunk - Non default index",
			config: &cfg{
				event:      "This is my new event! And some number 101",
				index:      integrationtestutils.GetConfigVariable("EVENT_INDEX"),
				source:     "otel-source",
				sourcetype: "sck-otel-st",
			},
			startTime: "-1m@m",
			telType:   logsType,
		},
		{
			name: "Events to Splunk - metrics",
			config: &cfg{
				event:      "test.metric",
				index:      integrationtestutils.GetConfigVariable("METRIC_INDEX"),
				source:     "otel",
				sourcetype: "st-otel",
			},
			startTime: "",
			telType:   metricsType,
		},
		{
			name: "Events to Splunk - traces",
			config: &cfg{
				event:      "",
				index:      integrationtestutils.GetConfigVariable("TRACE_INDEX"),
				source:     "trace-source",
				sourcetype: "trace-sourcetype",
			},
			startTime: "-1m@m",
			telType:   tracesType,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			logger.Info("Test -> Splunk running at:", zap.String("host", integrationtestutils.GetConfigVariable("HOST")),
				zap.String("uiPort", integrationtestutils.GetConfigVariable("UI_PORT")),
				zap.String("hecPort", integrationtestutils.GetConfigVariable("HEC_PORT")),
				zap.String("managementPort", integrationtestutils.GetConfigVariable("MANAGEMENT_PORT")),
			)

			// Endpoint and Token do not have a default value so set them directly.
			config := NewFactory().CreateDefaultConfig().(*Config)
			config.Token = configopaque.String(integrationtestutils.GetConfigVariable("HEC_TOKEN"))
			config.Endpoint = "https://" + integrationtestutils.GetConfigVariable("HOST") + ":" + integrationtestutils.GetConfigVariable("HEC_PORT") + "/services/collector"
			config.Source = "otel"
			config.SourceType = "st-otel"

			if test.telType == metricsType {
				config.Index = test.config.index
			} else {
				config.Index = "main"
			}
			config.TLSSetting.InsecureSkipVerify = true

			url, err := config.getURL()
			require.NoError(t, err, "Must not error while getting URL")

			switch test.telType {
			case logsType:
				logsTest(t, config, url, test)
			case metricsType:
				metricsTest(t, config, url, test)
			case tracesType:
				tracesTest(t, config, url, test)
			default:
				assert.Fail(t, "Telemetry type must be set to one of the following values: metrics, traces, or logs.")
			}
		})
	}
}

func waitForEventToBeIndexed() {
	time.Sleep(3 * time.Second)
}
