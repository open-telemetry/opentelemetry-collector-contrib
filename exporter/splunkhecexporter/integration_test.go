// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"
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
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	integrationtestutils "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter/internal/integrationtestutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

type SplunkContainerConfig struct {
	conCtx    context.Context
	container testcontainers.Container
}

func setup() SplunkContainerConfig {
	// Perform setup operations here
	fmt.Println("Setting up...")
	cfg := startSplunk()
	integrationtestutils.CreateAnIndexInSplunk(integrationtestutils.GetConfigVariable("EVENT_INDEX"), "event")
	integrationtestutils.CreateAnIndexInSplunk(integrationtestutils.GetConfigVariable("METRIC_INDEX"), "metric")
	integrationtestutils.CreateAnIndexInSplunk(integrationtestutils.GetConfigVariable("TRACE_INDEX"), "event")
	fmt.Println("Index created")
	return cfg
}

func teardown(cfg SplunkContainerConfig) {
	// Perform teardown operations here
	fmt.Println("Tearing down...")
	// Stop and remove the container
	fmt.Println("Stopping container")
	err := cfg.container.Terminate(cfg.conCtx)
	if err != nil {
		fmt.Printf("Error while terminiating container")
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

func TestMain(m *testing.M) {
	splunkContCfg := setup()

	// Run the tests
	code := m.Run()

	teardown(splunkContCfg)
	// Exit with the test result code
	os.Exit(code)
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
		WaitingFor: wait.ForLog("Ansible playbook complete, will begin streaming splunkd_stderr.log").WithStartupTimeout(2 * time.Minute),
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
	logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
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
	logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
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

func TestSplunkHecExporterEventsToSplunk(t *testing.T) {
	logger := zaptest.NewLogger(t)
	logger.Info("Test -> Splunk running at:", zap.String("host", integrationtestutils.GetConfigVariable("HOST")),
		zap.String("uiPort", integrationtestutils.GetConfigVariable("UI_PORT")),
		zap.String("hecPort", integrationtestutils.GetConfigVariable("HEC_PORT")),
		zap.String("managementPort", integrationtestutils.GetConfigVariable("MANAGEMENT_PORT")),
	)
	// Endpoint and Token do not have a default value so set them directly.
	config := NewFactory().CreateDefaultConfig().(*Config)
	config.Token = configopaque.String(integrationtestutils.GetConfigVariable("HEC_TOKEN"))
	config.HTTPClientSettings.Endpoint = "https://" + integrationtestutils.GetConfigVariable("HOST") + ":" + integrationtestutils.GetConfigVariable("HEC_PORT") + "/services/collector"
	config.Source = "otel"
	config.SourceType = "st-otel"
	config.Index = "main"
	config.TLSSetting.InsecureSkipVerify = true

	url, err := config.getURL()
	require.NoError(t, err, "Must not error while getting URL")
	settings := exportertest.NewNopCreateSettings()
	c := newLogsClient(settings, config)
	logs := prepareLogs()
	httpClient := createInsecureClient()
	c.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(config, component.NewDefaultBuildInfo())}

	err = c.pushLogData(context.Background(), logs)
	require.NoError(t, err, "Must not error while sending Logs data")
	waitForEventToBeIndexed()

	query := "index=main *"
	events := integrationtestutils.CheckEventsFromSplunk(query, "-3h@h")
	logger.Info("Splunk received %d events in the last minute", zap.Int("no. of events", len(events)))
	assert.True(t, len(events) == 1)
	// check events fields
	data, ok := events[0].(map[string]any)
	if !ok {
		logger.Info("Invalid event format")
	}
	assert.True(t, "test log" == data["_raw"].(string))
	assert.True(t, "main" == data["index"].(string))
	assert.True(t, "otel" == data["source"].(string))
	assert.True(t, "st-otel" == data["sourcetype"].(string))
}

func TestSplunkHecExporterEventsToSplunkNonDefaultIndex(t *testing.T) {
	logger := zaptest.NewLogger(t)
	logger.Info("Test -> Splunk running at:", zap.String("host", integrationtestutils.GetConfigVariable("HOST")),
		zap.String("uiPort", integrationtestutils.GetConfigVariable("UI_PORT")),
		zap.String("hecPort", integrationtestutils.GetConfigVariable("HEC_PORT")),
		zap.String("managementPort", integrationtestutils.GetConfigVariable("MANAGEMENT_PORT")),
	)

	event := "This is my new event! And some number 101"
	index := integrationtestutils.GetConfigVariable("EVENT_INDEX")
	source := "otel-source"
	sourcetype := "sck-otel-st"

	// Endpoint and Token do not have a default value so set them directly.
	config := NewFactory().CreateDefaultConfig().(*Config)
	config.Token = configopaque.String(integrationtestutils.GetConfigVariable("HEC_TOKEN"))
	config.HTTPClientSettings.Endpoint = "https://" + integrationtestutils.GetConfigVariable("HOST") + ":" + integrationtestutils.GetConfigVariable("HEC_PORT") + "/services/collector"
	config.Source = "otel"
	config.SourceType = "st-otel"
	config.Index = "main"
	config.TLSSetting.InsecureSkipVerify = true

	url, err := config.getURL()
	require.NoError(t, err, "Must not error while getting URL")
	settings := exportertest.NewNopCreateSettings()
	c := newLogsClient(settings, config)
	logs := prepareLogsNonDefaultParams(index, source, sourcetype, event)
	httpClient := createInsecureClient()
	c.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(config, component.NewDefaultBuildInfo())}

	err = c.pushLogData(context.Background(), logs)
	require.NoError(t, err, "Must not error while sending Logs data")
	waitForEventToBeIndexed()

	query := "index=" + index + " *"
	events := integrationtestutils.CheckEventsFromSplunk(query, "-1m@m")
	logger.Info("Splunk received %d events in the last minute", zap.Int("no. of events", len(events)))
	assert.True(t, len(events) == 1)
	// check events fields
	data, ok := events[0].(map[string]any)
	if !ok {
		logger.Info("Invalid event format")
	}
	assert.True(t, event == data["_raw"].(string))
	assert.True(t, index == data["index"].(string))
	assert.True(t, source == data["source"].(string))
	assert.True(t, sourcetype == data["sourcetype"].(string))
}

func TestSplunkHecExporterMetricsToSplunk(t *testing.T) {
	logger := zaptest.NewLogger(t)
	logger.Info("Test -> Splunk running at:", zap.String("host", integrationtestutils.GetConfigVariable("HOST")),
		zap.String("uiPort", integrationtestutils.GetConfigVariable("UI_PORT")),
		zap.String("hecPort", integrationtestutils.GetConfigVariable("HEC_PORT")),
		zap.String("managementPort", integrationtestutils.GetConfigVariable("MANAGEMENT_PORT")),
	)
	index := integrationtestutils.GetConfigVariable("METRIC_INDEX")
	metricName := "test.metric"
	// Endpoint and Token do not have a default value so set them directly.
	config := NewFactory().CreateDefaultConfig().(*Config)
	config.Token = configopaque.String(integrationtestutils.GetConfigVariable("HEC_TOKEN"))
	config.HTTPClientSettings.Endpoint = "https://" + integrationtestutils.GetConfigVariable("HOST") + ":" + integrationtestutils.GetConfigVariable("HEC_PORT") + "/services/collector"
	config.Source = "otel"
	config.SourceType = "st-otel"
	config.Index = index
	config.TLSSetting.InsecureSkipVerify = true

	url, err := config.getURL()
	require.NoError(t, err, "Must not error while getting URL")
	settings := exportertest.NewNopCreateSettings()
	c := newMetricsClient(settings, config)
	metricData := prepareMetricsData(metricName)

	httpClient := createInsecureClient()
	c.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(config, component.NewDefaultBuildInfo())}

	err = c.pushMetricsData(context.Background(), metricData)
	require.NoError(t, err, "Must not error while sending Metrics data")
	waitForEventToBeIndexed()

	events := integrationtestutils.CheckMetricsFromSplunk(index, metricName)
	assert.True(t, len(events) == 1, "Events length is less than 1. No metrics found")
}

func TestSplunkHecExporterTracesToSplunk(t *testing.T) {
	logger := zaptest.NewLogger(t)
	logger.Info("Test -> Splunk running at:", zap.String("host", integrationtestutils.GetConfigVariable("HOST")),
		zap.String("uiPort", integrationtestutils.GetConfigVariable("UI_PORT")),
		zap.String("hecPort", integrationtestutils.GetConfigVariable("HEC_PORT")),
		zap.String("managementPort", integrationtestutils.GetConfigVariable("MANAGEMENT_PORT")),
	)
	index := integrationtestutils.GetConfigVariable("TRACE_INDEX")
	source := "trace-source"
	sourcetype := "trace-sourcetype"
	// Endpoint and Token do not have a default value so set them directly.
	config := NewFactory().CreateDefaultConfig().(*Config)
	config.Token = configopaque.String(integrationtestutils.GetConfigVariable("HEC_TOKEN"))
	config.HTTPClientSettings.Endpoint = "https://" + integrationtestutils.GetConfigVariable("HOST") + ":" + integrationtestutils.GetConfigVariable("HEC_PORT") + "/services/collector"
	config.Source = "otel"
	config.SourceType = "st-otel"
	config.Index = "main"
	config.TLSSetting.InsecureSkipVerify = true

	url, err := config.getURL()
	require.NoError(t, err, "Must not error while getting URL")
	settings := exportertest.NewNopCreateSettings()
	c := newTracesClient(settings, config)
	tracesData := prepareTracesData(index, source, sourcetype)

	httpClient := createInsecureClient()
	c.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(config, component.NewDefaultBuildInfo())}

	err = c.pushTraceData(context.Background(), tracesData)
	require.NoError(t, err, "Must not error while sending Trace data")

	waitForEventToBeIndexed()
	query := "index=" + index + " *"
	events := integrationtestutils.CheckEventsFromSplunk(query, "-1m@m")
	logger.Info("Splunk received %d events in the last minute", zap.Int("no. of events", len(events)))
	assert.True(t, len(events) == 1)
	// check fields
	data, ok := events[0].(map[string]any)
	if !ok {
		logger.Info("Invalid event format")
	}
	assert.True(t, index == data["index"].(string))
	assert.True(t, source == data["source"].(string))
	assert.True(t, sourcetype == data["sourcetype"].(string))
}

func waitForEventToBeIndexed() {
	time.Sleep(3 * time.Second)
}
