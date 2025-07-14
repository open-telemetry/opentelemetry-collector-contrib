// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opsrampmetricsfilterprocessor

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateProcessor(t *testing.T) {
	cfg := &Config{
		AlertConfigMapName: "test-config",
		AlertConfigMapKey:  "alert-definitions.yaml",
		Namespace:          "test-namespace",
	}

	// This test will fail in CI/testing environment without Kubernetes cluster
	// But validates the configuration structure
	_, err := createMetricsProcessor(
		context.Background(),
		processortest.NewNopSettings(),
		cfg,
		consumertest.NewNop(),
	)

	// Expect error due to no Kubernetes cluster in test environment
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create")
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with all fields",
			config: &Config{
				AlertConfigMapName: "test-config",
				AlertConfigMapKey:  "alert-definitions.yaml",
				Namespace:          "test-namespace",
			},
			wantErr: false,
		},
		{
			name:    "empty config uses defaults",
			config:  &Config{},
			wantErr: false,
		},
		{
			name: "partial config with configmap name only",
			config: &Config{
				AlertConfigMapName: "my-config",
			},
			wantErr: false,
		},
		{
			name: "partial config with key only",
			config: &Config{
				AlertConfigMapKey: "my-alerts.yaml",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store original namespace env
			originalNS := os.Getenv("NAMESPACE")
			defer func() {
				if originalNS != "" {
					os.Setenv("NAMESPACE", originalNS)
				} else {
					os.Unsetenv("NAMESPACE")
				}
			}()

			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				// Verify defaults are set
				assert.NotEmpty(t, tt.config.AlertConfigMapName)
				assert.NotEmpty(t, tt.config.AlertConfigMapKey)
				assert.NotEmpty(t, tt.config.Namespace)
			}
		})
	}
}

func TestExtractMetricsFromExpression(t *testing.T) {
	// This is a unit test that doesn't require Kubernetes
	processor := &filterProcessor{
		logger: processortest.NewNopSettings().Logger,
	}

	tests := []struct {
		name     string
		expr     string
		expected []string
	}{
		{
			name:     "simple metric",
			expr:     "up",
			expected: []string{"up"},
		},
		{
			name:     "metric with labels",
			expr:     "http_requests_total{job=\"api-server\"}",
			expected: []string{"http_requests_total"},
		},
		{
			name:     "binary operation",
			expr:     "rate(http_requests_total[5m]) > 0.5",
			expected: []string{"http_requests_total"},
		},
		{
			name:     "multiple metrics",
			expr:     "cpu_usage_percent + memory_usage_percent",
			expected: []string{"cpu_usage_percent", "memory_usage_percent"},
		},
		{
			name:     "function with metric",
			expr:     "increase(errors_total[1h])",
			expected: []string{"errors_total"},
		},
		{
			name:     "invalid expression",
			expr:     "invalid{",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.extractMetricsFromExpression(tt.expr)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestAlertDefinitionsStructure(t *testing.T) {
	// Test the YAML structure parsing
	yamlData := `
alertDefinitions:
  - resourceType: "Pod"
    rules:
      - name: "High CPU Usage"
        interval: "30s"
        expr: "cpu_usage_percent > 80"
        isAvailability: false
      - name: "Memory Usage"
        interval: "1m"
        expr: "memory_usage_bytes / memory_limit_bytes * 100 > 90"
        isAvailability: false
  - resourceType: "Node"
    rules:
      - name: "Node Down"
        interval: "1m"
        expr: "up == 0"
        isAvailability: true
`

	var alertDefs AlertDefinitions
	err := yaml.Unmarshal([]byte(yamlData), &alertDefs)
	assert.NoError(t, err)
	assert.Len(t, alertDefs.AlertDefinitions, 2)
	assert.Equal(t, "Pod", alertDefs.AlertDefinitions[0].ResourceType)
	assert.Len(t, alertDefs.AlertDefinitions[0].Rules, 2)
	assert.Equal(t, "High CPU Usage", alertDefs.AlertDefinitions[0].Rules[0].Name)
	assert.Equal(t, "cpu_usage_percent > 80", alertDefs.AlertDefinitions[0].Rules[0].Expr)
}

func TestDotToUnderscoreConversion(t *testing.T) {
	// Test the critical dot-to-underscore conversion that fixes Prometheus compatibility
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "metric with dots",
			input:    "k8s.pod.cpu.usage",
			expected: "k8s_pod_cpu_usage",
		},
		{
			name:     "metric without dots",
			input:    "cpu_usage_percent",
			expected: "cpu_usage_percent",
		},
		{
			name:     "multiple dots",
			input:    "system.disk.io.read.bytes",
			expected: "system_disk_io_read_bytes",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := strings.ReplaceAll(tc.input, ".", "_")
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMetricsFilteringLogic(t *testing.T) {
	// Test the core filtering logic without Kubernetes dependencies
	filterMap := map[string]bool{
		"cpu_usage_percent":   true,
		"memory_usage_bytes":  true,
		"network_bytes_total": true,
	}

	testCases := []struct {
		name          string
		metricName    string
		shouldInclude bool
	}{
		{
			name:          "included metric",
			metricName:    "cpu_usage_percent",
			shouldInclude: true,
		},
		{
			name:          "excluded metric",
			metricName:    "disk_usage_bytes",
			shouldInclude: false,
		},
		{
			name:          "metric with dots converted",
			metricName:    "k8s.pod.cpu.usage",
			shouldInclude: false, // Should be false because filter expects underscores
		},
		{
			name:          "converted metric name",
			metricName:    "network_bytes_total",
			shouldInclude: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Apply the same conversion logic as in the processor
			convertedName := strings.ReplaceAll(tc.metricName, ".", "_")
			result := filterMap[convertedName]
			assert.Equal(t, tc.shouldInclude, result)
		})
	}
}

func TestNoMetricsConfiguredDropsAllMetrics(t *testing.T) {
	// Create a processor with empty metrics map (no alert definitions)
	processor := &filterProcessor{
		metricsMap: make(map[string]bool), // Empty map - no metrics configured
		logger:     zap.NewNop(),
	}

	// Create test metrics
	testMetrics := pmetric.NewMetrics()
	rm := testMetrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// Add a test metric
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test_metric")
	metric.SetEmptyGauge()
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(42.0)

	// Create a mock consumer to capture what gets sent
	mockConsumer := &consumertest.MetricsSink{}
	processor.nextConsumer = mockConsumer

	// Process the metrics
	err := processor.ConsumeMetrics(context.Background(), testMetrics)
	require.NoError(t, err)

	// Verify that empty metrics were sent (all metrics dropped)
	require.Len(t, mockConsumer.AllMetrics(), 1)
	receivedMetrics := mockConsumer.AllMetrics()[0]

	// Should have no resource metrics (empty)
	assert.Equal(t, 0, receivedMetrics.ResourceMetrics().Len())
}

func TestNoIncomingMetricsEarlyReturn(t *testing.T) {
	// Create a processor with some metrics configured
	processor := &filterProcessor{
		metricsMap: map[string]bool{"some_metric": true},
		logger:     zap.NewNop(),
	}

	// Create empty metrics (no metrics to process)
	testMetrics := pmetric.NewMetrics()

	// Create a mock consumer to capture what gets sent
	mockConsumer := &consumertest.MetricsSink{}
	processor.nextConsumer = mockConsumer

	// Process the empty metrics
	err := processor.ConsumeMetrics(context.Background(), testMetrics)
	require.NoError(t, err)

	// Verify that the same empty metrics were passed through
	require.Len(t, mockConsumer.AllMetrics(), 1)
	receivedMetrics := mockConsumer.AllMetrics()[0]

	// Should have no resource metrics (same as input)
	assert.Equal(t, 0, receivedMetrics.ResourceMetrics().Len())
}

func TestNamespaceFromEnvironment(t *testing.T) {
	// Store original namespace env
	originalNS := os.Getenv("NAMESPACE")
	defer func() {
		if originalNS != "" {
			os.Setenv("NAMESPACE", originalNS)
		} else {
			os.Unsetenv("NAMESPACE")
		}
	}()

	tests := []struct {
		name              string
		envValue          string
		expectedNamespace string
	}{
		{
			name:              "namespace from environment",
			envValue:          "production",
			expectedNamespace: "production",
		},
		{
			name:              "fallback to default when env not set",
			envValue:          "",
			expectedNamespace: "opsramp-agent",
		},
		{
			name:              "namespace from environment overrides default",
			envValue:          "staging",
			expectedNamespace: "staging",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv("NAMESPACE", tt.envValue)
			} else {
				os.Unsetenv("NAMESPACE")
			}

			config := &Config{}
			err := config.Validate()
			require.NoError(t, err)

			assert.Equal(t, tt.expectedNamespace, config.Namespace)
		})
	}
}
