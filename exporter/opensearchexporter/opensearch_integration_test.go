// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package opensearchexporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
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

func setupOpenSearch(t *testing.T, securityEnabled bool) string {
	t.Helper()

	env := map[string]string{
		"discovery.type":              "single-node",
		"DISABLE_INSTALL_DEMO_CONFIG": "true",
	}
	if !securityEnabled {
		env["DISABLE_SECURITY_PLUGIN"] = "true"
	}

	req := testcontainers.ContainerRequest{
		Image:        "opensearchproject/opensearch:3.6.0",
		ExposedPorts: []string{"9200/tcp"},
		Env:          env,
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
		require.NoError(t, container.Terminate(t.Context()))
	})

	host, err := container.Host(t.Context())
	require.NoError(t, err)

	port, err := container.MappedPort(t.Context(), "9200/tcp")
	require.NoError(t, err)

	return fmt.Sprintf("http://%s:%s", host, port.Port())
}

func TestIntegration_OtelV1Mapping_Traces(t *testing.T) {
	testCases := []struct {
		name            string
		securityEnabled bool
	}{
		{"security_disabled", false},
		{"security_enabled", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := setupOpenSearch(t, tc.securityEnabled)

			client, err := opensearchapi.NewClient(opensearchapi.Config{Client: opensearch.Config{Addresses: []string{endpoint}}})
			require.NoError(t, err)

			cfg := NewFactory().CreateDefaultConfig().(*Config)
			cfg.Endpoint = endpoint
			cfg.TLS.Insecure = true
			cfg.Mode = "otel-v1"
			// Fix CI race condition by disabling queue soConsumeTraces blocks until complete
			cfg.QueueConfig = configoptional.None[exporterhelper.QueueBatchConfig]()

			require.NoError(t, cfg.Validate())

			exporter, err := NewFactory().CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
			require.NoError(t, err)
			require.NoError(t, exporter.Start(t.Context(), componenttest.NewNopHost()))
			t.Cleanup(func() {
				require.NoError(t, exporter.Shutdown(t.Context()))
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

			require.Eventually(t, func() bool {
				_, err = client.Indices.Refresh(t.Context(), &opensearchapi.IndicesRefreshReq{Indices: []string{"otel-v1-apm-span"}})
				if err != nil {
					return false
				}

				mappingResp, err := client.Indices.Mapping.Get(t.Context(), &opensearchapi.MappingGetReq{Indices: []string{"otel-v1-apm-span"}})
				if err != nil || len(mappingResp.Indices) == 0 {
					return false
				}

				var responseMap map[string]any
				if err := json.NewDecoder(bytes.NewReader(mappingResp.Indices["otel-v1-apm-span"].Mappings)).Decode(&responseMap); err != nil {
					return false
				}

				propertiesMap, ok := responseMap["properties"].(map[string]any)
				if !ok {
					return false
				}

				succeeded := true
				func() {
					defer func() {
						if recover() != nil {
							succeeded = false
						}
					}()

					// Verify dynamic long mappings
					require.Equal(t, "long", propertiesMap["durationInNanos"].(map[string]any)["type"])

					statusProps := propertiesMap["status"].(map[string]any)
					require.Equal(t, "long", statusProps["properties"].(map[string]any)["code"].(map[string]any)["type"])
				}()

				return succeeded
			}, 10*time.Second, 500*time.Millisecond)
		})
	}
}

func TestIntegration_OtelV1Mapping_Logs(t *testing.T) {
	endpoint := setupOpenSearch(t, false)

	client, err := opensearchapi.NewClient(opensearchapi.Config{Client: opensearch.Config{Addresses: []string{endpoint}}})
	require.NoError(t, err)

	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Endpoint = endpoint
	cfg.TLS.Insecure = true
	cfg.Mode = "otel-v1"
	// Fix CI race condition by disabling queue so ConsumeLogs blocks until complete
	cfg.QueueConfig = configoptional.None[exporterhelper.QueueBatchConfig]()

	require.NoError(t, cfg.Validate())

	exporter, err := NewFactory().CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, exporter.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, exporter.Shutdown(t.Context()))
	})

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "my-test-service")
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("This is a test log message")
	logRecord.SetSeverityNumber(plog.SeverityNumberFatal)

	require.NoError(t, exporter.ConsumeLogs(t.Context(), logs))

	require.Eventually(t, func() bool {
		_, err = client.Indices.Refresh(t.Context(), &opensearchapi.IndicesRefreshReq{Indices: []string{"logs-otel-v1"}})
		if err != nil {
			return false
		}

		mappingResp, err := client.Indices.Mapping.Get(t.Context(), &opensearchapi.MappingGetReq{Indices: []string{"logs-otel-v1"}})
		if err != nil || len(mappingResp.Indices) == 0 {
			return false
		}

		var responseMap map[string]any
		if err := json.NewDecoder(bytes.NewReader(mappingResp.Indices["logs-otel-v1"].Mappings)).Decode(&responseMap); err != nil {
			return false
		}

		propertiesMap, ok := responseMap["properties"].(map[string]any)
		if !ok {
			return false
		}

		succeeded := true
		func() {
			defer func() {
				if recover() != nil {
					succeeded = false
				}
			}()

			// Verify severityNumber maps as long
			require.Equal(t, "long", propertiesMap["severityNumber"].(map[string]any)["type"])
		}()

		// TODO(#48585): assert @timestamp == "date_nanos" once the manage_index_template PR lands.
		return succeeded
	}, 10*time.Second, 500*time.Millisecond)
}
