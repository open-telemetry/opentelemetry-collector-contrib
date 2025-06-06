// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package payload // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSplitPayloadInterface(t *testing.T) {
	payload := &OtelCollectorPayload{}
	_, err := payload.SplitPayload(1)
	require.Error(t, err)
	require.ErrorContains(t, err, payloadSplitErr)
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
	require.Equal(t, oc.Hostname, unmarshaled.Hostname)
	require.Equal(t, oc.UUID, unmarshaled.UUID)
	require.Equal(t, oc.Metadata.FullComponents, unmarshaled.Metadata.FullComponents)
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
	require.Equal(t, payload, unmarshaledPayload)
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
	require.Equal(t, hostname, testPayload.Hostname)
	require.NotZero(t, testPayload.Timestamp)
	require.Equal(t, extensionUUID, testPayload.UUID)

	// Test metadata fields
	require.Equal(t, hostname, testPayload.Metadata.Hostname)
	require.Equal(t, hostnameSource, testPayload.Metadata.HostnameSource)
	require.Equal(t, hostname+"-"+extensionUUID, testPayload.Metadata.CollectorID)
	require.Equal(t, version, testPayload.Metadata.CollectorVersion)
	require.Equal(t, site, testPayload.Metadata.ConfigSite)
	require.Equal(t, buildInfo, testPayload.Metadata.BuildInfo)
	require.Equal(t, fullConfig, testPayload.Metadata.FullConfiguration)
	require.Contains(t, testPayload.Metadata.HealthStatus, "healthy")

	// Test full components
	require.Len(t, testPayload.Metadata.FullComponents, 6)
	require.Contains(t, testPayload.Metadata.FullComponents, fullComponents[0]) // otlp receiver
	require.Contains(t, testPayload.Metadata.FullComponents, fullComponents[1]) // hostmetrics receiver
	require.Contains(t, testPayload.Metadata.FullComponents, fullComponents[2]) // batch processor
	require.Contains(t, testPayload.Metadata.FullComponents, fullComponents[3]) // memory_limiter processor
	require.Contains(t, testPayload.Metadata.FullComponents, fullComponents[4]) // debug exporter
	require.Contains(t, testPayload.Metadata.FullComponents, fullComponents[5]) // otlphttp exporter

	// Test active components
	require.Len(t, testPayload.Metadata.ActiveComponents, 7)

	// Verify that each component type appears in the expected pipelines
	tracesPipelineComponents := 0
	metricsPipelineComponents := 0

	for _, comp := range testPayload.Metadata.ActiveComponents {
		require.NotEmpty(t, comp.ID)
		require.NotEmpty(t, comp.Type)
		require.NotEmpty(t, comp.Kind)
		require.NotEmpty(t, comp.Pipeline)
		require.NotEmpty(t, comp.Gomod)
		require.NotEmpty(t, comp.Version)
		require.Equal(t, "active", comp.ComponentStatus)

		switch comp.Pipeline {
		case "traces":
			tracesPipelineComponents++
		case "metrics":
			metricsPipelineComponents++
		default:
		}
	}

	// Verify pipeline component counts
	require.Equal(t, 3, tracesPipelineComponents, "traces pipeline should have 3 components (otlp, batch, debug)")
	require.Equal(t, 4, metricsPipelineComponents, "metrics pipeline should have 4 components (otlp, hostmetrics, batch, debug)")

	// Test JSON serialization
	jsonData, err := testPayload.MarshalJSON()
	require.NoError(t, err)
	require.NotEmpty(t, jsonData)

	// Test that the JSON can be unmarshaled back
	var unmarshaledPayload OtelCollectorPayload
	err = json.Unmarshal(jsonData, &unmarshaledPayload)
	require.NoError(t, err)

	// Verify key fields after unmarshaling
	require.Equal(t, testPayload.Hostname, unmarshaledPayload.Hostname)
	require.Equal(t, testPayload.UUID, unmarshaledPayload.UUID)
	require.Equal(t, testPayload.Metadata.CollectorID, unmarshaledPayload.Metadata.CollectorID)
	require.Equal(t, testPayload.Metadata.CollectorVersion, unmarshaledPayload.Metadata.CollectorVersion)
	require.Len(t, unmarshaledPayload.Metadata.FullComponents, len(testPayload.Metadata.FullComponents))
	require.Len(t, unmarshaledPayload.Metadata.ActiveComponents, len(testPayload.Metadata.ActiveComponents))

	// Verify JSON structure contains expected top-level fields
	var jsonMap map[string]any
	err = json.Unmarshal(jsonData, &jsonMap)
	require.NoError(t, err)

	require.Contains(t, jsonMap, "hostname")
	require.Contains(t, jsonMap, "timestamp")
	require.Contains(t, jsonMap, "uuid")
	require.Contains(t, jsonMap, "otel_collector")

	// Verify metadata structure
	metadataMap, ok := jsonMap["otel_collector"].(map[string]any)
	require.True(t, ok, "metadata should be a map")
	require.Contains(t, metadataMap, "collector_id")
	require.Contains(t, metadataMap, "collector_version")
	require.Contains(t, metadataMap, "full_components")
	require.Contains(t, metadataMap, "active_components")
	require.Contains(t, metadataMap, "build_info")
	require.Contains(t, metadataMap, "full_configuration")
	require.Contains(t, metadataMap, "health_status")
}
