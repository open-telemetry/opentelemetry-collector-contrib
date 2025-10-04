// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTSDBSyncer(t *testing.T) {
	tests := []struct {
		name    string
		cfg     TSDBConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_config",
			cfg: TSDBConfig{
				QueryURL:       "http://prometheus:9090",
				RemoteWriteURL: "http://prometheus:9090/write",
				QueryTimeout:   30 * time.Second,
				WriteTimeout:   10 * time.Second,
				DedupWindow:    30 * time.Second,
				InstanceID:     "test",
			},
			wantErr: false,
		},
		{
			name: "missing_query_url",
			cfg: TSDBConfig{
				RemoteWriteURL: "http://prometheus:9090/write",
			},
			wantErr: true,
			errMsg:  "query_url required",
		},
		{
			name: "invalid_query_url",
			cfg: TSDBConfig{
				QueryURL: "://invalid",
			},
			wantErr: true,
			errMsg:  "invalid query_url",
		},
		{
			name: "invalid_write_url",
			cfg: TSDBConfig{
				QueryURL:       "http://prometheus:9090",
				RemoteWriteURL: "://invalid",
			},
			wantErr: true,
			errMsg:  "invalid remote_write_url",
		},
		{
			name: "default_timeouts",
			cfg: TSDBConfig{
				QueryURL: "http://prometheus:9090",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syncer, err := NewTSDBSyncer(tt.cfg)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, syncer)

				// Check defaults
				if tt.name == "default_timeouts" {
					assert.Equal(t, 30*time.Second, syncer.client.Timeout)
					assert.Equal(t, 30*time.Second, syncer.dedupWindow)
					assert.Equal(t, "unknown", syncer.instanceID)
				}
			}
		})
	}
}

func TestQueryActive(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/query", r.URL.Path)

		query := r.URL.Query().Get("query")
		assert.Contains(t, query, `otel_alert_active{rule_id="test_rule"} != 0`)

		response := vmResp{
			Status: "success",
			Data: vmData{
				ResultType: "vector",
				Result: []vmResult{
					{
						Metric: map[string]string{
							"__name__":             "otel_alert_active",
							"rule_id":              "test_rule",
							"service":              "api",
							"severity":             "warning",
							"for_duration_seconds": "60",
						},
						Value: vmValue{"1234567890", "1"},
					},
					{
						Metric: map[string]string{
							"__name__": "otel_alert_active",
							"rule_id":  "test_rule",
							"service":  "db",
							"severity": "critical",
						},
						Value: vmValue{"1234567890", "0"},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	syncer, err := NewTSDBSyncer(TSDBConfig{
		QueryURL:     server.URL,
		QueryTimeout: 5 * time.Second,
	})
	require.NoError(t, err)

	entries, err := syncer.QueryActive("test_rule")
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	// Check entries
	for fp, entry := range entries {
		assert.NotZero(t, fp)
		if entry.Labels["service"] == "api" {
			assert.True(t, entry.Active)
			assert.Equal(t, 60*time.Second, entry.ForDuration)
		} else {
			assert.False(t, entry.Active)
		}
	}
}

func TestQueryActiveErrors(t *testing.T) {
	t.Run("server_error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		syncer, _ := NewTSDBSyncer(TSDBConfig{
			QueryURL:     server.URL,
			QueryTimeout: 1 * time.Second,
		})

		_, err := syncer.QueryActive("test_rule")
		assert.Error(t, err)
	})

	t.Run("query_failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := vmResp{
				Status: "error",
				Error:  "query failed",
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		syncer, _ := NewTSDBSyncer(TSDBConfig{
			QueryURL:     server.URL,
			QueryTimeout: 1 * time.Second,
		})

		_, err := syncer.QueryActive("test_rule")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "query failed")
	})

	t.Run("nil_syncer", func(t *testing.T) {
		var syncer *TSDBSyncer
		entries, err := syncer.QueryActive("test")
		assert.NoError(t, err)
		assert.Empty(t, entries)
	})
}

func TestPublishEvents(t *testing.T) {
	t.Run("successful_publish", func(t *testing.T) {
		var receivedData []byte
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/write", r.URL.Path)
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/x-protobuf", r.Header.Get("Content-Type"))
			assert.Equal(t, "snappy", r.Header.Get("Content-Encoding"))

			// Capture the data for verification
			receivedData, _ = io.ReadAll(r.Body)

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		syncer, err := NewTSDBSyncer(TSDBConfig{
			QueryURL:       "http://test",
			RemoteWriteURL: server.URL + "/write",
			WriteTimeout:   5 * time.Second,
			InstanceID:     "test-instance",
		})
		require.NoError(t, err)

		events := []interface{}{
			AlertEvent{
				Rule:        "test_rule",
				State:       "firing",
				Severity:    "warning",
				Labels:      map[string]string{"service": "api"},
				Value:       0.95,
				Window:      "5m",
				For:         "1m",
				Timestamp:   time.Now(),
				Fingerprint: 12345,
			},
			AlertEvent{
				Rule:        "test_rule",
				State:       "resolved",
				Severity:    "critical",
				Labels:      map[string]string{"service": "db"},
				Value:       0.5,
				Window:      "5m",
				For:         "1m",
				Timestamp:   time.Now(),
				Fingerprint: 67890,
			},
		}

		err = syncer.PublishEvents(events)
		require.NoError(t, err)
		assert.NotEmpty(t, receivedData)
	})

	t.Run("publish_with_map_events", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		syncer, _ := NewTSDBSyncer(TSDBConfig{
			QueryURL:       "http://test",
			RemoteWriteURL: server.URL,
			InstanceID:     "test",
		})

		events := []interface{}{
			map[string]interface{}{
				"rule":        "test_rule",
				"state":       "firing",
				"severity":    "warning",
				"labels":      map[string]string{"test": "label"},
				"value":       1.0,
				"window":      "5m",
				"for":         "1m",
				"fingerprint": uint64(12345),
			},
		}

		err := syncer.PublishEvents(events)
		require.NoError(t, err)
	})

	t.Run("server_error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		syncer, _ := NewTSDBSyncer(TSDBConfig{
			QueryURL:       "http://test",
			RemoteWriteURL: server.URL,
		})

		events := []interface{}{
			AlertEvent{
				Rule:  "test",
				State: "firing",
			},
		}

		err := syncer.PublishEvents(events)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "remote write failed")
	})

	t.Run("nil_syncer", func(t *testing.T) {
		var syncer *TSDBSyncer
		err := syncer.PublishEvents([]interface{}{})
		assert.NoError(t, err)
	})

	t.Run("write_disabled", func(t *testing.T) {
		syncer := &TSDBSyncer{
			enableWrite: false,
		}
		err := syncer.PublishEvents([]interface{}{AlertEvent{}})
		assert.NoError(t, err)
	})

	t.Run("empty_events", func(t *testing.T) {
		syncer, _ := NewTSDBSyncer(TSDBConfig{
			QueryURL:       "http://test",
			RemoteWriteURL: "http://test/write",
		})
		err := syncer.PublishEvents([]interface{}{})
		assert.NoError(t, err)
	})
}

func TestConvertMapToAlertEvent(t *testing.T) {
	syncer := &TSDBSyncer{}

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected *AlertEvent
	}{
		{
			name: "full_conversion",
			input: map[string]interface{}{
				"rule":        "test_rule",
				"state":       "firing",
				"severity":    "critical",
				"value":       0.95,
				"window":      "5m",
				"for":         "1m",
				"fingerprint": uint64(12345),
				"labels": map[string]string{
					"service": "api",
					"env":     "prod",
				},
			},
			expected: &AlertEvent{
				Rule:        "test_rule",
				State:       "firing",
				Severity:    "critical",
				Value:       0.95,
				Window:      "5m",
				For:         "1m",
				Fingerprint: 12345,
				Labels: map[string]string{
					"service": "api",
					"env":     "prod",
				},
			},
		},
		{
			name: "labels_as_interface_map",
			input: map[string]interface{}{
				"rule": "test",
				"labels": map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expected: &AlertEvent{
				Rule: "test",
				Labels: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
		{
			name: "generate_fingerprint",
			input: map[string]interface{}{
				"rule": "test_rule",
				"labels": map[string]string{
					"service": "api",
				},
			},
			expected: &AlertEvent{
				Rule: "test_rule",
				Labels: map[string]string{
					"service": "api",
				},
			},
		},
		{
			name:     "empty_map",
			input:    map[string]interface{}{},
			expected: &AlertEvent{Labels: map[string]string{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncer.convertMapToAlertEvent(tt.input)
			require.NotNil(t, result)

			assert.Equal(t, tt.expected.Rule, result.Rule)
			assert.Equal(t, tt.expected.State, result.State)
			assert.Equal(t, tt.expected.Severity, result.Severity)
			assert.Equal(t, tt.expected.Value, result.Value)
			assert.Equal(t, tt.expected.Window, result.Window)
			assert.Equal(t, tt.expected.For, result.For)
			assert.Equal(t, tt.expected.Labels, result.Labels)

			// Check fingerprint generation
			if tt.name == "generate_fingerprint" {
				assert.NotZero(t, result.Fingerprint)
			}
		})
	}
}

func TestFingerprint(t *testing.T) {
	tests := []struct {
		name   string
		rule   string
		labels map[string]string
	}{
		{
			name:   "simple",
			rule:   "test_rule",
			labels: map[string]string{"service": "api"},
		},
		{
			name:   "multiple_labels",
			rule:   "test_rule",
			labels: map[string]string{"service": "api", "env": "prod", "region": "us-east"},
		},
		{
			name:   "empty_labels",
			rule:   "test_rule",
			labels: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp1 := fingerprint(tt.rule, tt.labels)
			fp2 := fingerprint(tt.rule, tt.labels)

			// Should be consistent
			assert.Equal(t, fp1, fp2)

			// Should be non-zero
			assert.NotZero(t, fp1)

			// Different rule should give different fingerprint
			fp3 := fingerprint("different_rule", tt.labels)
			assert.NotEqual(t, fp1, fp3)

			// Different labels should give different fingerprint
			if len(tt.labels) > 0 {
				modifiedLabels := make(map[string]string)
				for k, v := range tt.labels {
					modifiedLabels[k] = v + "_modified"
				}
				fp4 := fingerprint(tt.rule, modifiedLabels)
				assert.NotEqual(t, fp1, fp4)
			}
		})
	}
}

func TestLegacyConstructor(t *testing.T) {
	syncer, err := NewTSDBSyncerLegacy(
		"http://prometheus:9090",
		30*time.Second,
		60*time.Second,
	)

	require.NoError(t, err)
	assert.NotNil(t, syncer)
	assert.Equal(t, "legacy", syncer.instanceID)
	assert.Equal(t, 60*time.Second, syncer.dedupWindow)
}
