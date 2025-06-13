// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package payload // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitPayloadInterface(t *testing.T) {
	payload := &OtelCollectorPayload{}
	_, err := payload.SplitPayload(1)
	require.Error(t, err)
	assert.ErrorContains(t, err, payloadSplitErr)
}

func TestOtelCollectorPayload_MarshalJSON(t *testing.T) {
	oc := &OtelCollectorPayload{
		Hostname:  "test_host",
		Timestamp: time.Now().UnixNano(),
		UUID:      "test-uuid",
		Metadata: OtelCollector{
			FullComponents: []CollectorModule{
				{
					Type:       "test_type",
					Kind:       "test_kind",
					Gomod:      "test_gomod",
					Version:    "test_version",
					Configured: true,
				},
			},
		},
	}

	jsonData, err := json.Marshal(oc)
	require.NoError(t, err)

	var unmarshaled OtelCollectorPayload
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, oc.Hostname, unmarshaled.Hostname)
	assert.Equal(t, oc.UUID, unmarshaled.UUID)
	assert.Equal(t, oc.Metadata.FullComponents, unmarshaled.Metadata.FullComponents)
}

func TestOtelCollectorPayload_UnmarshalAndMarshal(t *testing.T) {
	// Read the sample JSON payload from file
	filePath := "testdata/sample-otelcollectorpayload.json"
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	// Unmarshal the JSON into an OtelCollectorPayload struct
	var payload OtelCollectorPayload
	err = json.Unmarshal(data, &payload)
	require.NoError(t, err)

	// Marshal the struct back into JSON
	marshaledJSON, err := json.Marshal(payload)
	require.NoError(t, err)

	// Unmarshal the marshaled JSON back into a struct
	var unmarshaledPayload OtelCollectorPayload
	err = json.Unmarshal(marshaledJSON, &unmarshaledPayload)
	require.NoError(t, err)

	// Assert that the original and unmarshaled structs are equal
	assert.Equal(t, payload, unmarshaledPayload)
}

// TestCompleteOtelCollectorPayload tests the creation of a full OtelCollectorPayload
// with realistic data including active components, full components, and health status
func TestCompleteOtelCollectorPayload(t *testing.T) {
	// Create build info
	buildInfo := CustomBuildInfo{
		Command:     "otelcol-contrib",
		Description: "OpenTelemetry Collector Contrib",
		Version:     "0.127.0",
	}

	// Create realistic configuration data
	fullConfig := `{
		"receivers": {
			"otlp": {
				"protocols": {
					"grpc": {},
					"http": {}
				}
			},
			"hostmetrics": {
				"collection_interval": "30s",
				"scrapers": ["cpu", "memory", "disk"]
			}
		},
		"processors": {
			"batch": {
				"send_batch_size": 1024,
				"timeout": "10s"
			},
			"memory_limiter": {
				"check_interval": "1s",
				"limit_mib": 4000
			}
		},
		"exporters": {
			"debug": {
				"verbosity": "detailed"
			},
			"otlphttp": {
				"endpoint": "http://localhost:4318"
			}
		},
		"service": {
			"pipelines": {
				"traces": {
					"receivers": ["otlp"],
					"processors": ["memory_limiter", "batch"],
					"exporters": ["debug", "otlphttp"]
				},
				"metrics": {
					"receivers": ["otlp", "hostmetrics"],
					"processors": ["memory_limiter", "batch"],
					"exporters": ["debug", "otlphttp"]
				}
			}
		}
	}`

	// Test data
	hostname := "test-unit-host"
	hostnameSource := "config"
	extensionUUID := "unit-test-uuid-67890"
	version := "0.127.0"
	site := "datadoghq.com"

	// Prepare base metadata
	metadata := PrepareOtelCollectorPayload(
		hostname,
		hostnameSource,
		extensionUUID,
		version,
		site,
		fullConfig,
		buildInfo,
	)

	// Add full components (what would come from module info)
	fullComponents := []CollectorModule{
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
	}
	metadata.FullComponents = fullComponents

	// Add active components (what would come from active component detection)
	activeComponents := []ServiceComponent{
		{
			ID:              "receiver/otlp",
			Name:            "otlp",
			Type:            "otlp",
			Kind:            "receiver",
			Pipeline:        "traces",
			Gomod:           "go.opentelemetry.io/collector/receiver/otlpreceiver",
			Version:         "v0.127.0",
			ComponentStatus: "active",
		},
		{
			ID:              "receiver/otlp",
			Name:            "otlp",
			Type:            "otlp",
			Kind:            "receiver",
			Pipeline:        "metrics",
			Gomod:           "go.opentelemetry.io/collector/receiver/otlpreceiver",
			Version:         "v0.127.0",
			ComponentStatus: "active",
		},
		{
			ID:              "receiver/hostmetrics",
			Name:            "hostmetrics",
			Type:            "hostmetrics",
			Kind:            "receiver",
			Pipeline:        "metrics",
			Gomod:           "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver",
			Version:         "v0.127.0",
			ComponentStatus: "active",
		},
		{
			ID:              "processor/batch",
			Name:            "batch",
			Type:            "batch",
			Kind:            "processor",
			Pipeline:        "traces",
			Gomod:           "go.opentelemetry.io/collector/processor/batchprocessor",
			Version:         "v0.127.0",
			ComponentStatus: "active",
		},
		{
			ID:              "processor/batch",
			Name:            "batch",
			Type:            "batch",
			Kind:            "processor",
			Pipeline:        "metrics",
			Gomod:           "go.opentelemetry.io/collector/processor/batchprocessor",
			Version:         "v0.127.0",
			ComponentStatus: "active",
		},
		{
			ID:              "exporter/debug",
			Name:            "debug",
			Type:            "debug",
			Kind:            "exporter",
			Pipeline:        "traces",
			Gomod:           "go.opentelemetry.io/collector/exporter/debugexporter",
			Version:         "v0.127.0",
			ComponentStatus: "active",
		},
		{
			ID:              "exporter/debug",
			Name:            "debug",
			Type:            "debug",
			Kind:            "exporter",
			Pipeline:        "metrics",
			Gomod:           "go.opentelemetry.io/collector/exporter/debugexporter",
			Version:         "v0.127.0",
			ComponentStatus: "active",
		},
	}
	metadata.ActiveComponents = activeComponents

	// Add health status
	metadata.HealthStatus = `{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `","uptime":"5m30s"}`

	// Create final payload
	testPayload := &OtelCollectorPayload{
		Hostname:  hostname,
		Timestamp: time.Now().UnixNano(),
		UUID:      extensionUUID,
		Metadata:  metadata,
	}

	// Test that all fields are properly set
	assert.Equal(t, hostname, testPayload.Hostname)
	assert.NotZero(t, testPayload.Timestamp)
	assert.Equal(t, extensionUUID, testPayload.UUID)

	// Test metadata fields
	assert.Equal(t, hostname, testPayload.Metadata.Hostname)
	assert.Equal(t, hostnameSource, testPayload.Metadata.HostnameSource)
	assert.Equal(t, hostname+"-"+extensionUUID, testPayload.Metadata.CollectorID)
	assert.Equal(t, version, testPayload.Metadata.CollectorVersion)
	assert.Equal(t, site, testPayload.Metadata.ConfigSite)
	assert.Equal(t, buildInfo, testPayload.Metadata.BuildInfo)
	assert.Equal(t, fullConfig, testPayload.Metadata.FullConfiguration)
	assert.Contains(t, testPayload.Metadata.HealthStatus, "healthy")

	// Test full components
	assert.Len(t, testPayload.Metadata.FullComponents, 6)
	assert.Contains(t, testPayload.Metadata.FullComponents, fullComponents[0]) // otlp receiver
	assert.Contains(t, testPayload.Metadata.FullComponents, fullComponents[1]) // hostmetrics receiver
	assert.Contains(t, testPayload.Metadata.FullComponents, fullComponents[2]) // batch processor
	assert.Contains(t, testPayload.Metadata.FullComponents, fullComponents[3]) // memory_limiter processor
	assert.Contains(t, testPayload.Metadata.FullComponents, fullComponents[4]) // debug exporter
	assert.Contains(t, testPayload.Metadata.FullComponents, fullComponents[5]) // otlphttp exporter

	// Test active components
	assert.Len(t, testPayload.Metadata.ActiveComponents, 7)

	// Verify that each component type appears in the expected pipelines
	tracesPipelineComponents := 0
	metricsPipelineComponents := 0

	for _, comp := range testPayload.Metadata.ActiveComponents {
		assert.NotEmpty(t, comp.ID)
		assert.NotEmpty(t, comp.Type)
		assert.NotEmpty(t, comp.Kind)
		assert.NotEmpty(t, comp.Pipeline)
		assert.NotEmpty(t, comp.Gomod)
		assert.NotEmpty(t, comp.Version)
		assert.Equal(t, "active", comp.ComponentStatus)

		switch comp.Pipeline {
		case "traces":
			tracesPipelineComponents++
		case "metrics":
			metricsPipelineComponents++
		default:
		}
	}

	// Verify pipeline component counts
	assert.Equal(t, 3, tracesPipelineComponents, "traces pipeline should have 3 components (otlp, batch, debug)")
	assert.Equal(t, 4, metricsPipelineComponents, "metrics pipeline should have 4 components (otlp, hostmetrics, batch, debug)")

	// Test JSON serialization
	jsonData, err := testPayload.MarshalJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test that the JSON can be unmarshaled back
	var unmarshaledPayload OtelCollectorPayload
	err = json.Unmarshal(jsonData, &unmarshaledPayload)
	require.NoError(t, err)

	// Verify key fields after unmarshaling
	assert.Equal(t, testPayload.Hostname, unmarshaledPayload.Hostname)
	assert.Equal(t, testPayload.UUID, unmarshaledPayload.UUID)
	assert.Equal(t, testPayload.Metadata.CollectorID, unmarshaledPayload.Metadata.CollectorID)
	assert.Equal(t, testPayload.Metadata.CollectorVersion, unmarshaledPayload.Metadata.CollectorVersion)
	assert.Len(t, unmarshaledPayload.Metadata.FullComponents, len(testPayload.Metadata.FullComponents))
	assert.Len(t, unmarshaledPayload.Metadata.ActiveComponents, len(testPayload.Metadata.ActiveComponents))

	// Verify JSON structure contains expected top-level fields
	var jsonMap map[string]any
	err = json.Unmarshal(jsonData, &jsonMap)
	require.NoError(t, err)

	assert.Contains(t, jsonMap, "hostname")
	assert.Contains(t, jsonMap, "timestamp")
	assert.Contains(t, jsonMap, "uuid")
	assert.Contains(t, jsonMap, "otel_collector")

	// Verify metadata structure
	metadataMap, ok := jsonMap["otel_collector"].(map[string]any)
	assert.True(t, ok, "metadata should be a map")
	assert.Contains(t, metadataMap, "collector_id")
	assert.Contains(t, metadataMap, "collector_version")
	assert.Contains(t, metadataMap, "full_components")
	assert.Contains(t, metadataMap, "active_components")
	assert.Contains(t, metadataMap, "build_info")
	assert.Contains(t, metadataMap, "full_configuration")
	assert.Contains(t, metadataMap, "health_status")
}
