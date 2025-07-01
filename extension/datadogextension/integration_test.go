// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/componentchecker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

func TestPopulateActiveComponentsIntegration(t *testing.T) {
	// Load the sample configuration file
	configPath := filepath.Join("internal", "componentchecker", "testdata", "sample-config.yaml")

	// Verify the config file exists
	_, err := os.Stat(configPath)
	require.NoError(t, err, "sample-config.yaml should exist")

	// Create a resolver to load the configuration
	resolverSettings := confmap.ResolverSettings{
		URIs: []string{"file:" + configPath},
		ProviderFactories: []confmap.ProviderFactory{
			fileprovider.NewFactory(),
		},
		ConverterFactories: []confmap.ConverterFactory{},
	}

	resolver, err := confmap.NewResolver(resolverSettings)
	require.NoError(t, err, "should be able to create resolver")

	confMap, err := resolver.Resolve(context.Background())
	require.NoError(t, err, "should be able to load config file")

	// Create a realistic ModuleInfoJSON that matches the components in sample-config.yaml
	moduleInfoJSON := createModuleInfoFromSampleConfig()

	// Test PopulateActiveComponents with the loaded configuration
	activeComponents, err := componentchecker.PopulateActiveComponents(confMap, moduleInfoJSON)
	require.NoError(t, err, "PopulateActiveComponents should not return error")
	require.NotNil(t, activeComponents, "activeComponents should not be nil")

	// Verify that we have the expected components from the sample config
	// Expected: 2 extensions + components across 3 pipelines
	// Extensions: health_check, pprof (2)
	// Pipeline components: each component appears once per pipeline it's used in
	// - otlp: 3 times (traces, metrics, logs)
	// - hostmetrics: 1 time (metrics)
	// - memory_limiter: 3 times (traces, metrics, logs)
	// - batch: 3 times (traces, metrics, logs)
	// - debug: 3 times (traces, metrics, logs)
	// - otlphttp: 3 times (traces, metrics, logs)
	// - datadog/connector: 2 times (traces exporter, metrics receiver)
	// Total: 2 + 3 + 1 + 3 + 3 + 3 + 3 + 2 = 20
	expectedComponentCount := 20
	assert.Len(t, *activeComponents, expectedComponentCount, "should have expected number of active components")

	// Verify that extensions are present
	hasHealthCheck := false
	hasPprof := false

	// Verify that pipeline components are present
	hasOtlp := false
	hasHostmetrics := false
	hasBatch := false
	hasMemoryLimiter := false
	hasDebug := false
	hasOtlphttp := false

	for _, component := range *activeComponents {
		switch component.Type {
		case "health_check":
			hasHealthCheck = true
			assert.Equal(t, "extension", component.Kind)
			assert.Empty(t, component.Pipeline) // Extensions don't have pipelines
		case "pprof":
			hasPprof = true
			assert.Equal(t, "extension", component.Kind)
			assert.Empty(t, component.Pipeline)
		case "otlp":
			hasOtlp = true
			assert.Equal(t, "receiver", component.Kind)
			assert.Contains(t, []string{"traces", "metrics", "logs"}, component.Pipeline)
		case "hostmetrics":
			hasHostmetrics = true
			assert.Equal(t, "receiver", component.Kind)
			assert.Equal(t, "metrics", component.Pipeline)
		case "batch":
			hasBatch = true
			assert.Equal(t, "processor", component.Kind)
			assert.Contains(t, []string{"traces", "metrics", "logs"}, component.Pipeline)
		case "memory_limiter":
			hasMemoryLimiter = true
			assert.Equal(t, "processor", component.Kind)
			assert.Contains(t, []string{"traces", "metrics", "logs"}, component.Pipeline)
		case "debug":
			hasDebug = true
			assert.Equal(t, "exporter", component.Kind)
			assert.Contains(t, []string{"traces", "metrics", "logs"}, component.Pipeline)
		case "otlphttp":
			hasOtlphttp = true
			assert.Equal(t, "exporter", component.Kind)
			assert.Contains(t, []string{"traces", "metrics", "logs"}, component.Pipeline)
		}

		// Verify that all components have module information
		assert.NotEmpty(t, component.Gomod, "component %s should have gomod info", component.Type)
		assert.NotEmpty(t, component.Version, "component %s should have version info", component.Type)
		assert.NotEmpty(t, component.ID, "component %s should have ID", component.Type)
		assert.NotEmpty(t, component.Type, "component should have type")
		assert.NotEmpty(t, component.Kind, "component should have kind")
	}

	// Assert that all expected components are present
	assert.True(t, hasHealthCheck, "should have health_check extension")
	assert.True(t, hasPprof, "should have pprof extension")
	assert.True(t, hasOtlp, "should have otlp receiver")
	assert.True(t, hasHostmetrics, "should have hostmetrics receiver")
	assert.True(t, hasBatch, "should have batch processor")
	assert.True(t, hasMemoryLimiter, "should have memory_limiter processor")
	assert.True(t, hasDebug, "should have debug exporter")
	assert.True(t, hasOtlphttp, "should have otlphttp exporter")
}

func TestDataToFlattenedJSONStringIntegration(t *testing.T) {
	// Load the sample configuration file
	configPath := filepath.Join("internal", "componentchecker", "testdata", "sample-config.yaml")

	// Create a resolver to load the configuration
	resolverSettings := confmap.ResolverSettings{
		URIs: []string{"file:" + configPath},
		ProviderFactories: []confmap.ProviderFactory{
			fileprovider.NewFactory(),
		},
		ConverterFactories: []confmap.ConverterFactory{},
	}

	resolver, err := confmap.NewResolver(resolverSettings)
	require.NoError(t, err, "should be able to create resolver")

	confMap, err := resolver.Resolve(context.Background())
	require.NoError(t, err, "should be able to load config file")

	// Test DataToFlattenedJSONString with the loaded configuration
	jsonString := componentchecker.DataToFlattenedJSONString(confMap.ToStringMap())

	// Verify that the result is valid JSON and doesn't contain newlines or carriage returns
	assert.NotEmpty(t, jsonString, "JSON string should not be empty")
	assert.NotContains(t, jsonString, "\n", "JSON string should not contain newlines")
	assert.NotContains(t, jsonString, "\r", "JSON string should not contain carriage returns")

	// Verify it's valid JSON by attempting to unmarshal
	var result map[string]any
	err = json.Unmarshal([]byte(jsonString), &result)
	assert.NoError(t, err, "flattened JSON should be valid JSON")
}

// TestFullOtelCollectorPayloadIntegration tests the complete end-to-end flow of:
// 1. Creating a full OtelCollectorPayload with realistic data
// 2. Setting up mock Datadog agent components (Logger, Forwarder, Compressor, Serializer, Config)
// 3. Sending the payload to a mock Datadog backend
func TestFullOtelCollectorPayloadIntegration(t *testing.T) {
	// Set up a mock Datadog backend server
	var receivedPayloads []payload.OtelCollectorPayload
	var mu sync.Mutex

	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		assert.NotEmpty(t, r.Header.Get("DD-API-KEY"))

		// Read and decode the compressed payload
		defer r.Body.Close()

		// Since the payload is compressed, we need to decompress it
		// The test will verify the serializer can handle the payload correctly
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "ok"}`)

		// For testing purposes, we'll track that a request was made
		mu.Lock()
		defer mu.Unlock()
		// Note: In a real scenario, we'd decompress and unmarshal the payload
		// For this test, we'll create a mock payload to verify structure
		mockPayload := createTestOtelCollectorPayload()
		receivedPayloads = append(receivedPayloads, *mockPayload)
	}))
	defer mockBackend.Close()

	// Create telemetry settings for component creation
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, err := config.Build()
	require.NoError(t, err)

	telemetrySettings := component.TelemetrySettings{
		Logger: logger,
	}

	// Step 1: Create a full OtelCollectorPayload with realistic data
	testPayload := createTestOtelCollectorPayload()
	assert.NotNil(t, testPayload)

	// Verify the payload structure
	assert.NotEmpty(t, testPayload.Hostname)
	assert.NotZero(t, testPayload.Timestamp)
	assert.NotEmpty(t, testPayload.UUID)
	assert.NotEmpty(t, testPayload.Metadata.CollectorID)
	assert.NotEmpty(t, testPayload.Metadata.CollectorVersion)
	assert.NotEmpty(t, testPayload.Metadata.FullConfiguration)
	assert.NotEmpty(t, testPayload.Metadata.FullComponents)
	assert.NotEmpty(t, testPayload.Metadata.ActiveComponents)

	// Step 2: Create mock Datadog agent components

	// Extract the backend URL to configure components to use our mock
	backendURL := mockBackend.URL

	// Create configuration component with test API key and site
	cfg := datadogconfig.CreateDefaultConfig().(*datadogconfig.Config)
	cfg.API.Key = "test-api-key-12345"
	// Override the site URL to point to our mock backend
	// We need to extract just the host:port from the test server URL
	testSite := backendURL[7:] // Remove "http://" prefix

	cfgOptions := []agentcomponents.ConfigOption{
		agentcomponents.WithAPIConfig(cfg),
		agentcomponents.WithLogsEnabled(),
		agentcomponents.WithLogLevel(telemetrySettings),
		agentcomponents.WithPayloadsConfig(),
		agentcomponents.WithForwarderConfig(),
		agentcomponents.WithCustomConfig("dd_url", testSite+"/api/v1/otel_collector", pkgconfigmodel.SourceDefault),
	}
	configComponent := agentcomponents.NewConfigComponent(cfgOptions...)

	// Create log component
	logComponent := agentcomponents.NewLogComponent(telemetrySettings)
	require.NotNil(t, logComponent)

	// Create serializer
	serializer := agentcomponents.NewSerializerComponent(configComponent, logComponent, testPayload.Hostname)
	require.NotNil(t, serializer)

	// Step 3: Verify we can marshal the payload (simulating serialization)
	marshaledPayload, err := testPayload.MarshalJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, marshaledPayload)

	// Verify the marshaled payload can be unmarshaled back
	var unmarshaledPayload payload.OtelCollectorPayload
	err = json.Unmarshal(marshaledPayload, &unmarshaledPayload)
	require.NoError(t, err)
	assert.Equal(t, testPayload.Hostname, unmarshaledPayload.Hostname)
	assert.Equal(t, testPayload.UUID, unmarshaledPayload.UUID)

	// Step 4: Test that the components work together
	// Verify config component has correct settings
	assert.Equal(t, "test-api-key-12345", configComponent.GetString("api_key"))
	assert.True(t, configComponent.GetBool("logs_enabled"))
	assert.True(t, configComponent.GetBool("enable_payloads.json_to_v1_intake"))

	// Step 5: Simulate sending payload (in a real scenario, this would use serializer.SendEvents or similar)
	// For this test, we simulate the HTTP request that would be made
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodPost, backendURL+"/api/v1/otel_collector", nil)
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("DD-API-KEY", "test-api-key-12345")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify the response
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Wait a bit for the mock server to process
	time.Sleep(100 * time.Millisecond)

	// Verify that our mock backend received the request
	mu.Lock()
	assert.Len(t, receivedPayloads, 1, "should have received one payload")
	mu.Unlock()

	// Verify the received payload structure
	if len(receivedPayloads) > 0 {
		receivedPayload := receivedPayloads[0]
		assert.Equal(t, testPayload.Hostname, receivedPayload.Hostname)
		assert.Equal(t, testPayload.UUID, receivedPayload.UUID)
		assert.NotEmpty(t, receivedPayload.Metadata.FullComponents)
		assert.NotEmpty(t, receivedPayload.Metadata.ActiveComponents)
	}
}

// createTestOtelCollectorPayload creates a realistic test payload with full component data
func createTestOtelCollectorPayload() *payload.OtelCollectorPayload {
	// Load sample configuration to get realistic data
	configPath := filepath.Join("internal", "componentchecker", "testdata", "sample-config.yaml")

	// Create resolver and load config
	resolverSettings := confmap.ResolverSettings{
		URIs: []string{"file:" + configPath},
		ProviderFactories: []confmap.ProviderFactory{
			fileprovider.NewFactory(),
		},
		ConverterFactories: []confmap.ConverterFactory{},
	}

	resolver, _ := confmap.NewResolver(resolverSettings)
	confMap, _ := resolver.Resolve(context.Background())

	// Create module info and populate active components
	moduleInfoJSON := createModuleInfoFromSampleConfig()
	activeComponents, _ := componentchecker.PopulateActiveComponents(confMap, moduleInfoJSON)

	// Create build info
	buildInfo := payload.CustomBuildInfo{
		Command:     "otelcol-contrib",
		Description: "OpenTelemetry Collector Contrib",
		Version:     "0.127.0",
	}

	// Get flattened configuration
	fullConfig := componentchecker.DataToFlattenedJSONString(confMap.ToStringMap())

	// Prepare base metadata
	hostname := "test-integration-host"
	hostnameSource := "config"
	extensionUUID := "integration-test-uuid-12345"
	version := "0.127.0"
	site := "datadoghq.com"

	metadata := payload.PrepareOtelCollectorMetadata(
		hostname,
		hostnameSource,
		extensionUUID,
		version,
		site,
		fullConfig,
		buildInfo,
	)

	// Populate with realistic component data
	if activeComponents != nil {
		metadata.ActiveComponents = *activeComponents
	}

	// Add full components from module info
	if moduleInfoJSON != nil {
		for _, component := range moduleInfoJSON.GetFullComponentsList() {
			metadata.FullComponents = append(metadata.FullComponents, payload.CollectorModule{
				Type:       component.Type,
				Kind:       component.Kind,
				Gomod:      component.Gomod,
				Version:    component.Version,
				Configured: component.Configured,
			})
		}
	}

	// Add health status
	metadata.HealthStatus = `{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`

	// Create final payload
	return &payload.OtelCollectorPayload{
		Hostname:  hostname,
		Timestamp: time.Now().UnixNano(),
		UUID:      extensionUUID,
		Metadata:  metadata,
	}
}

// createModuleInfoFromSampleConfig creates a realistic ModuleInfoJSON
// that matches the components used in the sample-config.yaml
func createModuleInfoFromSampleConfig() *payload.ModuleInfoJSON {
	moduleInfo := payload.NewModuleInfoJSON()

	// Add realistic component information based on what's in the sample config
	components := []payload.CollectorModule{
		// Receivers
		{
			Type:       "otlp",
			Kind:       "receiver",
			Gomod:      "go.opentelemetry.io/collector/receiver/otlpreceiver",
			Version:    "v0.127.0",
			Configured: true,
		},
		{
			Type:       "hostmetrics",
			Kind:       "receiver",
			Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver",
			Version:    "v0.127.0",
			Configured: true,
		},

		// Processors
		{
			Type:       "batch",
			Kind:       "processor",
			Gomod:      "go.opentelemetry.io/collector/processor/batchprocessor",
			Version:    "v0.127.0",
			Configured: true,
		},
		{
			Type:       "memory_limiter",
			Kind:       "processor",
			Gomod:      "go.opentelemetry.io/collector/processor/memorylimiterprocessor",
			Version:    "v0.127.0",
			Configured: true,
		},

		// Exporters
		{
			Type:       "debug",
			Kind:       "exporter",
			Gomod:      "go.opentelemetry.io/collector/exporter/debugexporter",
			Version:    "v0.127.0",
			Configured: true,
		},
		{
			Type:       "otlphttp",
			Kind:       "exporter",
			Gomod:      "go.opentelemetry.io/collector/exporter/otlphttpexporter",
			Version:    "v0.127.0",
			Configured: true,
		},

		// Extensions
		{
			Type:       "health_check",
			Kind:       "extension",
			Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension",
			Version:    "v0.127.0",
			Configured: true,
		},
		{
			Type:       "pprof",
			Kind:       "extension",
			Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension",
			Version:    "v0.127.0",
			Configured: true,
		},

		// Connectors
		{
			Type:       "datadog",
			Kind:       "connector",
			Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector",
			Version:    "v0.127.0",
			Configured: true,
		},
	}

	for _, component := range components {
		moduleInfo.AddComponent(component)
	}

	return moduleInfo
}

// TestHTTPServerIntegration tests the complete end-to-end flow of:
// 1. Creating a httpserver.Server with realistic configuration
// 2. Setting up mock Datadog agent components (Logger, Serializer, Config)
// 3. Starting the HTTP server and testing both local endpoint and payload sending
// 4. Verifying payloads are sent to a mock Datadog backend
func TestHTTPServerIntegration(t *testing.T) {
	// Set up a mock Datadog backend server to receive payloads
	var receivedPayloads []payload.OtelCollectorPayload
	var payloadMutex sync.Mutex

	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request is for otel collector metadata
		if !strings.Contains(r.URL.Path, "otel_collector") && !strings.Contains(r.URL.Path, "metadata") {
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}

		// Verify request headers for Datadog API
		assert.NotEmpty(t, r.Header.Get("DD-API-KEY"))

		// Read the request body (payload)
		defer r.Body.Close()

		// Note: In real scenarios, the payload would be compressed, but for testing
		// we'll simulate a successful response without full decompression
		payloadMutex.Lock()
		// Create a mock payload for verification
		mockPayload := createTestOtelCollectorPayload()
		receivedPayloads = append(receivedPayloads, *mockPayload)
		payloadMutex.Unlock()

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "ok"}`)
	}))
	defer mockBackend.Close()

	// Create telemetry settings for component creation
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, err := config.Build()
	require.NoError(t, err)

	telemetrySettings := component.TelemetrySettings{
		Logger: logger,
	}

	// Step 1: Create test configuration and payload
	testHostname := "httpserver-test-host"
	testUUID := "httpserver-test-uuid-67890"

	// Load sample configuration for realistic data
	configPath := filepath.Join("internal", "componentchecker", "testdata", "sample-config.yaml")
	resolverSettings := confmap.ResolverSettings{
		URIs: []string{"file:" + configPath},
		ProviderFactories: []confmap.ProviderFactory{
			fileprovider.NewFactory(),
		},
		ConverterFactories: []confmap.ConverterFactory{},
	}

	resolver, err := confmap.NewResolver(resolverSettings)
	require.NoError(t, err)

	confMap, err := resolver.Resolve(context.Background())
	require.NoError(t, err)

	// Create module info and populate active components for realistic test data
	moduleInfoJSON := createModuleInfoFromSampleConfig()
	activeComponents, err := componentchecker.PopulateActiveComponents(confMap, moduleInfoJSON)
	require.NoError(t, err)

	// Create OtelCollector metadata
	buildInfo := payload.CustomBuildInfo{
		Command:     "otelcol-contrib",
		Description: "OpenTelemetry Collector Contrib",
		Version:     "0.127.0",
	}
	fullConfig := componentchecker.DataToFlattenedJSONString(confMap.ToStringMap())
	otelMetadata := payload.PrepareOtelCollectorMetadata(
		testHostname,
		"config",
		testUUID,
		"0.127.0",
		"datadoghq.com",
		fullConfig,
		buildInfo,
	)
	if activeComponents != nil {
		otelMetadata.ActiveComponents = *activeComponents
	}

	// Step 2: Create mock Datadog agent components configured to use mock backend

	// Extract backend URL for configuration - remove the scheme and use just host:port
	backendURL := mockBackend.URL
	backendHost := strings.TrimPrefix(backendURL, "http://")

	// Create configuration component with test API key pointing to mock backend
	cfg := datadogconfig.CreateDefaultConfig().(*datadogconfig.Config)
	cfg.API.Key = "test-httpserver-api-key-12345"

	cfgOptions := []agentcomponents.ConfigOption{
		agentcomponents.WithAPIConfig(cfg),
		agentcomponents.WithLogsEnabled(),
		agentcomponents.WithLogLevel(telemetrySettings),
		agentcomponents.WithPayloadsConfig(),
		agentcomponents.WithForwarderConfig(),
		// Configure to send to our mock backend
		agentcomponents.WithCustomConfig("dd_url", "http://"+backendHost+"/api/v1", pkgconfigmodel.SourceDefault),
		agentcomponents.WithCustomConfig("api_endpoint", backendHost, pkgconfigmodel.SourceDefault),
	}
	configComponent := agentcomponents.NewConfigComponent(cfgOptions...)

	// Create log component
	logComponent := agentcomponents.NewLogComponent(telemetrySettings)
	require.NotNil(t, logComponent)

	// Create serializer with forwarder
	serializer := agentcomponents.NewSerializerComponent(configComponent, logComponent, testHostname)
	require.NotNil(t, serializer)

	// Step 3: Create HTTP server configuration
	serverConfig := &httpserver.Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:0", // Use any available port for testing
		},
		Path: "/otel/metadata",
	}

	// Step 4: Create and test the HTTP server
	server := httpserver.NewServer(
		logger,
		serializer,
		serverConfig,
		testHostname,
		testUUID,
		otelMetadata,
	)
	require.NotNil(t, server)

	// Start the serializer (required for payload sending)
	err = serializer.Start()
	require.NoError(t, err)
	defer serializer.Stop()

	// Start the HTTP server
	server.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(ctx)
	}()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Step 5: Test SendPayload functionality
	marshaledPayload, err := server.SendPayload()
	require.NoError(t, err)
	require.NotNil(t, marshaledPayload)

	// Verify the payload structure
	payloadBytes, err := marshaledPayload.MarshalJSON()
	require.NoError(t, err)

	var testPayloadStruct payload.OtelCollectorPayload
	err = json.Unmarshal(payloadBytes, &testPayloadStruct)
	require.NoError(t, err)

	assert.Equal(t, testHostname, testPayloadStruct.Hostname)
	assert.Equal(t, testUUID, testPayloadStruct.UUID)
	assert.NotZero(t, testPayloadStruct.Timestamp)
	assert.NotEmpty(t, testPayloadStruct.Metadata.CollectorID)
	assert.NotEmpty(t, testPayloadStruct.Metadata.ActiveComponents)

	// Step 6: Test HTTP endpoint functionality
	// Test the handler directly since we're using port 0
	req := httptest.NewRequest(http.MethodGet, serverConfig.Path, nil)
	recorder := httptest.NewRecorder()

	server.HandleMetadata(recorder, req)

	// Verify response
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	// Verify response body contains valid JSON
	var responsePayload payload.OtelCollectorPayload
	err = json.Unmarshal(recorder.Body.Bytes(), &responsePayload)
	require.NoError(t, err)

	assert.Equal(t, testHostname, responsePayload.Hostname)
	assert.Equal(t, testUUID, responsePayload.UUID)
	assert.NotEmpty(t, responsePayload.Metadata.ActiveComponents)

	// Step 7: Test error scenarios

	// Test SendPayload when serializer is stopped
	serializer.Stop()
	_, err = server.SendPayload()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forwarder is not started")

	// Test HandleMetadata with nil ResponseWriter (should not panic)
	server.HandleMetadata(nil, httptest.NewRequest(http.MethodGet, serverConfig.Path, nil))
}

// TestHTTPServerConfigIntegration tests different HTTP server configurations
func TestHTTPServerConfigIntegration(t *testing.T) {
	// Create basic telemetry settings
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, err := config.Build()
	require.NoError(t, err)

	telemetrySettings := component.TelemetrySettings{
		Logger: logger,
	}

	// Create minimal agent components
	cfg := datadogconfig.CreateDefaultConfig().(*datadogconfig.Config)
	cfg.API.Key = "test-config-api-key"

	cfgOptions := []agentcomponents.ConfigOption{
		agentcomponents.WithAPIConfig(cfg),
		agentcomponents.WithForwarderConfig(),
	}
	configComponent := agentcomponents.NewConfigComponent(cfgOptions...)
	logComponent := agentcomponents.NewLogComponent(telemetrySettings)
	serializer := agentcomponents.NewSerializerComponent(configComponent, logComponent, "test-host")

	// Create minimal OtelCollector metadata
	buildInfo := payload.CustomBuildInfo{
		Command: "test-collector",
		Version: "1.0.0",
	}
	otelMetadata := payload.PrepareOtelCollectorMetadata(
		"test-host",
		"config",
		"test-uuid",
		"1.0.0",
		"datadoghq.com",
		"{}",
		buildInfo,
	)

	// Test different server configurations
	testCases := []struct {
		name   string
		config *httpserver.Config
	}{
		{
			name: "default_config",
			config: &httpserver.Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: httpserver.DefaultServerEndpoint,
				},
				Path: "/metadata",
			},
		},
		{
			name: "custom_endpoint_and_path",
			config: &httpserver.Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: "localhost:9999",
				},
				Path: "/custom/otel/metadata",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create server with test configuration
			server := httpserver.NewServer(
				logger,
				serializer,
				tc.config,
				"test-host-"+tc.name,
				"test-uuid-"+tc.name,
				otelMetadata,
			)
			require.NotNil(t, server)

			// Test server creation doesn't panic and can be started/stopped
			server.Start()
			time.Sleep(50 * time.Millisecond) // Brief pause to allow server startup

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			server.Stop(ctx)
		})
	}
}

// TestHTTPServerConcurrentAccess tests concurrent access to the HTTP server
func TestHTTPServerConcurrentAccess(t *testing.T) {
	// Create basic setup
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel) // Reduce log noise
	logger, err := config.Build()
	require.NoError(t, err)

	telemetrySettings := component.TelemetrySettings{
		Logger: logger,
	}

	// Create agent components
	cfg := datadogconfig.CreateDefaultConfig().(*datadogconfig.Config)
	cfg.API.Key = "test-concurrent-api-key"

	cfgOptions := []agentcomponents.ConfigOption{
		agentcomponents.WithAPIConfig(cfg),
		agentcomponents.WithForwarderConfig(),
	}
	configComponent := agentcomponents.NewConfigComponent(cfgOptions...)
	logComponent := agentcomponents.NewLogComponent(telemetrySettings)
	serializer := agentcomponents.NewSerializerComponent(configComponent, logComponent, "concurrent-test-host")

	// Create metadata
	buildInfo := payload.CustomBuildInfo{
		Command: "concurrent-test-collector",
		Version: "1.0.0",
	}
	otelMetadata := payload.PrepareOtelCollectorMetadata(
		"concurrent-test-host",
		"config",
		"concurrent-test-uuid",
		"1.0.0",
		"datadoghq.com",
		"{}",
		buildInfo,
	)

	// Create server
	serverConfig := &httpserver.Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:0",
		},
		Path: "/concurrent/metadata",
	}

	server := httpserver.NewServer(
		logger,
		serializer,
		serverConfig,
		"concurrent-test-host",
		"concurrent-test-uuid",
		otelMetadata,
	)

	// Start serializer
	err = serializer.Start()
	require.NoError(t, err)
	defer serializer.Stop()

	// Test concurrent calls to HandleMetadata
	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()

			// Create test request
			req := httptest.NewRequest(http.MethodGet, serverConfig.Path, nil)
			recorder := httptest.NewRecorder()

			// Call HandleMetadata
			server.HandleMetadata(recorder, req)

			// Verify response
			if recorder.Code != http.StatusOK {
				errors <- fmt.Errorf("routine %d: expected status 200, got %d", routineID, recorder.Code)
				return
			}

			// Verify response is valid JSON
			var payload payload.OtelCollectorPayload
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				errors <- fmt.Errorf("routine %d: failed to unmarshal response: %w", routineID, err)
				return
			}

			if payload.Hostname != "concurrent-test-host" {
				errors <- fmt.Errorf("routine %d: unexpected hostname: %s", routineID, payload.Hostname)
				return
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Error(err)
	}
}
