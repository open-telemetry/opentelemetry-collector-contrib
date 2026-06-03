// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package opensearchexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/metadata"
)

func setupOpenSearch(t *testing.T) string {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "opensearchproject/opensearch:2.11.0",
		ExposedPorts: []string{"9200/tcp"},
		Env: map[string]string{
			"discovery.type":          "single-node",
			"DISABLE_SECURITY_PLUGIN": "true",
		},
		WaitingFor: wait.ForHTTP("/_cluster/health").
			WithPort("9200/tcp").
			WithStatusCodeMatcher(func(status int) bool {
				return status == http.StatusOK
			}).
			WithStartupTimeout(2 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(t.Context(), testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, container.Terminate(context.Background())) //nolint:usetesting
	})

	host, err := container.Host(t.Context())
	require.NoError(t, err)

	port, err := container.MappedPort(t.Context(), "9200/tcp")
	require.NoError(t, err)

	return fmt.Sprintf("http://%s:%s", host, port.Port())
}

func TestIntegration_OtelV1Mapping_Traces(t *testing.T) {
	endpoint := setupOpenSearch(t)

	client, err := opensearchapi.NewClient(opensearchapi.Config{Client: opensearch.Config{Addresses: []string{endpoint}}})
	require.NoError(t, err)

	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Endpoint = endpoint
	cfg.TLS.Insecure = true
	cfg.Mode = "otel-v1"
	cfg.TracesIndex = "otel-v1-apm-span"
	cfg.QueueConfig = configoptional.None[exporterhelper.QueueBatchConfig]()

	require.NoError(t, cfg.Validate())

	exporter, err := NewFactory().CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, exporter.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, exporter.Shutdown(context.Background())) //nolint:usetesting
	})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "my-test-service")
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("integration-span")
	span.Attributes().PutInt("my_int_attr", 42)
	span.Attributes().PutDouble("my_double_attr", 3.14)
	span.SetParentSpanID(pcommon.NewSpanIDEmpty())

	startTime := time.Now()
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(startTime.Add(time.Second)))

	require.NoError(t, exporter.ConsumeTraces(t.Context(), traces))

	var lastErr error
	success := assert.Eventually(t, func() bool {
		_, err = client.Indices.Refresh(t.Context(), &opensearchapi.IndicesRefreshReq{Indices: []string{"otel-v1-apm-span"}})
		if err != nil {
			lastErr = fmt.Errorf("refresh error: %w", err)
			return false
		}

		mappingResp, err := client.Indices.Mapping.Get(t.Context(), &opensearchapi.MappingGetReq{Indices: []string{"otel-v1-apm-span"}})
		if err != nil || len(mappingResp.Indices) == 0 {
			lastErr = errors.New("index not found or mapping empty")
			return false
		}

		var responseMap map[string]any
		if err := json.NewDecoder(bytes.NewReader(mappingResp.Indices["otel-v1-apm-span"].Mappings)).Decode(&responseMap); err != nil {
			lastErr = fmt.Errorf("decode error: %w", err)
			return false
		}

		propertiesMap, ok := responseMap["properties"].(map[string]any)
		if !ok {
			lastErr = errors.New("no 'properties' found in mapping")
			return false
		}

		durMap, ok := propertiesMap["durationInNanos"].(map[string]any)
		if !ok {
			keys := []string{}
			for k := range propertiesMap {
				keys = append(keys, k)
			}
			lastErr = fmt.Errorf("durationInNanos not found. Available fields: %v", keys)
			return false
		}

		typeVal, _ := durMap["type"].(string)
		if typeVal == "long" || typeVal == "integer" {
			return true
		}

		lastErr = fmt.Errorf("unexpected type for durationInNanos: %s", typeVal)
		return false
	}, 30*time.Second, 500*time.Millisecond)

	if !success {
		t.Fatalf("Traces test failed. Last error: %v", lastErr)
	}
}

func TestIntegration_OtelV1Mapping_Logs(t *testing.T) {
	endpoint := setupOpenSearch(t)

	client, err := opensearchapi.NewClient(opensearchapi.Config{Client: opensearch.Config{Addresses: []string{endpoint}}})
	require.NoError(t, err)

	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Endpoint = endpoint
	cfg.TLS.Insecure = true
	cfg.Mode = "otel-v1"
	cfg.LogsIndex = "logs-otel-v1"
	cfg.QueueConfig = configoptional.None[exporterhelper.QueueBatchConfig]()

	require.NoError(t, cfg.Validate())

	exporter, err := NewFactory().CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, exporter.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, exporter.Shutdown(context.Background())) //nolint:usetesting
	})

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "my-test-service")
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("This is a test log message")
	logRecord.SetSeverityNumber(plog.SeverityNumberFatal)
	logRecord.SetSeverityText("FATAL")
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	require.NoError(t, exporter.ConsumeLogs(t.Context(), logs))

	var lastErr error
	success := assert.Eventually(t, func() bool {
		_, err = client.Indices.Refresh(t.Context(), &opensearchapi.IndicesRefreshReq{Indices: []string{"logs-otel-v1"}})
		if err != nil {
			lastErr = fmt.Errorf("refresh error: %w", err)
			return false
		}

		mappingResp, err := client.Indices.Mapping.Get(t.Context(), &opensearchapi.MappingGetReq{Indices: []string{"logs-otel-v1"}})
		if err != nil || len(mappingResp.Indices) == 0 {
			lastErr = errors.New("index not found or mapping empty")
			return false
		}

		var responseMap map[string]any
		if err := json.NewDecoder(bytes.NewReader(mappingResp.Indices["logs-otel-v1"].Mappings)).Decode(&responseMap); err != nil {
			lastErr = fmt.Errorf("decode error: %w", err)
			return false
		}

		propertiesMap, ok := responseMap["properties"].(map[string]any)
		if !ok {
			lastErr = errors.New("no 'properties' found in mapping")
			return false
		}

		fieldNames := []string{"severityNumber", "severity_number", "SeverityNumber", "severity"}
		var sevMap map[string]any
		for _, fn := range fieldNames {
			if sm, exists := propertiesMap[fn].(map[string]any); exists {
				sevMap = sm
				break
			}
		}

		if sevMap == nil {
			keys := []string{}
			for k := range propertiesMap {
				keys = append(keys, k)
			}
			lastErr = fmt.Errorf("severity field not found. Available fields in index: %v", keys)
			return false
		}

		return true
	}, 30*time.Second, 500*time.Millisecond)

	if !success {
		t.Fatalf("Logs test failed! Last error: %v", lastErr)
	}
}
