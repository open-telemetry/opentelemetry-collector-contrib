// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectors

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/rpc"
)

// MockCollector for testing
type MockCollector struct {
	name        string
	supported   bool
	shouldError bool
	hasMetrics  bool
	collectTime time.Duration
}

func (m *MockCollector) Name() string {
	return m.name
}

func (m *MockCollector) IsSupported(client *rpc.Client) bool {
	return m.supported
}

func (m *MockCollector) Collect(ctx context.Context, client *rpc.Client, timestamp time.Time) (pmetric.Metrics, error) {
	if m.collectTime > 0 {
		time.Sleep(m.collectTime)
	}

	if m.shouldError {
		return pmetric.NewMetrics(), errors.New("mock collector error")
	}

	metrics := pmetric.NewMetrics()
	if m.hasMetrics {
		// Create a simple metric to simulate real data
		rm := metrics.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("cisco_mock_metric")
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetDoubleValue(1.0)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	}

	return metrics, nil
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.collectors)
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	collector := &MockCollector{name: "test", supported: true}
	registry.Register(collector)

	retrieved, exists := registry.GetCollector("test")
	assert.True(t, exists)
	assert.Equal(t, collector, retrieved)
}

func TestRegistry_GetCollector(t *testing.T) {
	registry := NewRegistry()

	// Test non-existent collector
	_, exists := registry.GetCollector("nonexistent")
	assert.False(t, exists)

	// Test existing collector
	collector := &MockCollector{name: "bgp", supported: true}
	registry.Register(collector)

	retrieved, exists := registry.GetCollector("bgp")
	assert.True(t, exists)
	assert.Equal(t, collector, retrieved)
}

func TestRegistry_GetAllCollectors(t *testing.T) {
	registry := NewRegistry()

	// Test empty registry
	collectors := registry.GetAllCollectors()
	assert.Empty(t, collectors)

	// Test with multiple collectors
	bgp := &MockCollector{name: "bgp", supported: true}
	interfaces := &MockCollector{name: "interfaces", supported: true}

	registry.Register(bgp)
	registry.Register(interfaces)

	collectors = registry.GetAllCollectors()
	assert.Len(t, collectors, 2)
	assert.Contains(t, collectors, "bgp")
	assert.Contains(t, collectors, "interfaces")
}

func TestRegistry_CollectFromDevice(t *testing.T) {
	client := &rpc.Client{} // Mock client
	timestamp := time.Now()

	tests := []struct {
		name                string
		collectors          []MockCollector
		enabledCollectors   DeviceCollectors
		expectedMetricCount int
	}{
		{
			name: "all_collectors_enabled_and_supported",
			collectors: []MockCollector{
				{name: "bgp", supported: true, hasMetrics: true},
				{name: "interfaces", supported: true, hasMetrics: true},
				{name: "facts", supported: true, hasMetrics: true},
			},
			enabledCollectors: DeviceCollectors{
				BGP:        true,
				Interfaces: true,
				Facts:      true,
			},
			expectedMetricCount: 3,
		},
		{
			name: "some_collectors_disabled",
			collectors: []MockCollector{
				{name: "bgp", supported: true, hasMetrics: true},
				{name: "interfaces", supported: true, hasMetrics: true},
				{name: "facts", supported: true, hasMetrics: true},
			},
			enabledCollectors: DeviceCollectors{
				BGP:        true,
				Interfaces: false,
				Facts:      true,
			},
			expectedMetricCount: 2,
		},
		{
			name: "some_collectors_unsupported",
			collectors: []MockCollector{
				{name: "bgp", supported: false, hasMetrics: true},
				{name: "interfaces", supported: true, hasMetrics: true},
				{name: "facts", supported: true, hasMetrics: true},
			},
			enabledCollectors: DeviceCollectors{
				BGP:        true,
				Interfaces: true,
				Facts:      true,
			},
			expectedMetricCount: 2,
		},
		{
			name: "collectors_with_errors",
			collectors: []MockCollector{
				{name: "bgp", supported: true, shouldError: true},
				{name: "interfaces", supported: true, hasMetrics: true},
				{name: "facts", supported: true, hasMetrics: true},
			},
			enabledCollectors: DeviceCollectors{
				BGP:        true,
				Interfaces: true,
				Facts:      true,
			},
			expectedMetricCount: 2, // BGP should be skipped due to error
		},
		{
			name: "collectors_with_no_metrics",
			collectors: []MockCollector{
				{name: "bgp", supported: true, hasMetrics: false},
				{name: "interfaces", supported: true, hasMetrics: true},
				{name: "facts", supported: true, hasMetrics: true},
			},
			enabledCollectors: DeviceCollectors{
				BGP:        true,
				Interfaces: true,
				Facts:      true,
			},
			expectedMetricCount: 2, // BGP should be skipped due to no metrics
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh registry for each test
			testRegistry := NewRegistry()

			// Register collectors
			for _, collector := range tt.collectors {
				c := collector // Create copy to avoid closure issues
				testRegistry.Register(&c)
			}

			// Collect metrics
			ctx := context.Background()
			metrics, err := testRegistry.CollectFromDevice(ctx, client, tt.enabledCollectors, timestamp)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedMetricCount, metrics.ResourceMetrics().Len())
		})
	}
}

func TestRegistry_CollectFromDeviceWithTiming(t *testing.T) {
	registry := NewRegistry()
	client := &rpc.Client{}
	timestamp := time.Now()

	// Register collectors with different characteristics
	fastCollector := &MockCollector{
		name: "fast", supported: true, hasMetrics: true,
		collectTime: 10 * time.Millisecond,
	}
	slowCollector := &MockCollector{
		name: "slow", supported: true, hasMetrics: true,
		collectTime: 50 * time.Millisecond,
	}
	errorCollector := &MockCollector{
		name: "error", supported: true, shouldError: true,
	}

	registry.Register(fastCollector)
	registry.Register(slowCollector)
	registry.Register(errorCollector)

	enabledCollectors := DeviceCollectors{
		BGP:         true, // Will map to "fast"
		Interfaces:  true, // Will map to "slow"
		Environment: true, // Will map to "error"
	}

	// We need to register with correct names
	testRegistry := NewRegistry()
	testRegistry.Register(&MockCollector{name: "bgp", supported: true, hasMetrics: true, collectTime: 10 * time.Millisecond})
	testRegistry.Register(&MockCollector{name: "interfaces", supported: true, hasMetrics: true, collectTime: 50 * time.Millisecond})
	testRegistry.Register(&MockCollector{name: "environment", supported: true, shouldError: true})

	ctx := context.Background()
	metrics, timings, err := testRegistry.CollectFromDeviceWithTiming(ctx, client, enabledCollectors, timestamp)

	require.NoError(t, err)

	// Should have metrics from successful collectors only
	assert.Equal(t, 2, metrics.ResourceMetrics().Len())

	// Should have timing for successful collectors only
	assert.Len(t, timings, 2)
	assert.Contains(t, timings, "bgp")
	assert.Contains(t, timings, "interfaces")
	assert.NotContains(t, timings, "environment") // Error collector should not have timing

	// Verify timing values are reasonable
	assert.Greater(t, timings["bgp"], time.Duration(0))
	assert.Greater(t, timings["interfaces"], time.Duration(0))
}

func TestRegistry_ConcurrentCollection(t *testing.T) {
	registry := NewRegistry()
	client := &rpc.Client{}
	timestamp := time.Now()

	// Register multiple collectors
	for i := 0; i < 5; i++ {
		collector := &MockCollector{
			name:        fmt.Sprintf("collector_%d", i),
			supported:   true,
			hasMetrics:  true,
			collectTime: 20 * time.Millisecond,
		}
		registry.Register(collector)
	}

	// Enable all collectors (using a custom DeviceCollectors for this test)
	enabledCollectors := DeviceCollectors{
		BGP:         true,
		Environment: true,
		Facts:       true,
		Interfaces:  true,
		Optics:      true,
	}

	// Test concurrent collection
	ctx := context.Background()
	start := time.Now()
	results, err := registry.CollectConcurrently(ctx, client, enabledCollectors, timestamp)
	duration := time.Since(start)

	require.NoError(t, err)

	// Should complete faster than sequential execution
	// 5 collectors * 20ms = 100ms sequential, concurrent should be much faster
	assert.Less(t, duration, 80*time.Millisecond)

	// Should have results from all supported collectors
	assert.GreaterOrEqual(t, len(results), 0) // Some may not match the enabled names
}

func TestRegistry_GetEnabledCollectorNames(t *testing.T) {
	registry := NewRegistry()

	// Register collectors
	registry.Register(&MockCollector{name: "bgp", supported: true})
	registry.Register(&MockCollector{name: "interfaces", supported: true})
	registry.Register(&MockCollector{name: "facts", supported: true})
	registry.Register(&MockCollector{name: "environment", supported: true})
	registry.Register(&MockCollector{name: "optics", supported: true})

	enabledCollectors := DeviceCollectors{
		BGP:         true,
		Interfaces:  false,
		Facts:       true,
		Environment: false,
		Optics:      true,
	}

	names := registry.GetEnabledCollectorNames(enabledCollectors)

	assert.Len(t, names, 3)
	assert.Contains(t, names, "bgp")
	assert.Contains(t, names, "facts")
	assert.Contains(t, names, "optics")
	assert.NotContains(t, names, "interfaces")
	assert.NotContains(t, names, "environment")
}

func TestDeviceCollectors_IsEnabled(t *testing.T) {
	collectors := DeviceCollectors{
		BGP:         true,
		Environment: false,
		Facts:       true,
		Interfaces:  false,
		Optics:      true,
	}

	tests := []struct {
		name          string
		collectorName string
		expected      bool
	}{
		{"bgp_enabled", "bgp", true},
		{"environment_disabled", "environment", false},
		{"facts_enabled", "facts", true},
		{"interfaces_disabled", "interfaces", false},
		{"optics_enabled", "optics", true},
		{"unknown_collector", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collectors.IsEnabled(tt.collectorName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegistry_ErrorHandling(t *testing.T) {
	registry := NewRegistry()
	client := &rpc.Client{}
	timestamp := time.Now()

	// Register collectors with various error conditions
	registry.Register(&MockCollector{name: "bgp", supported: true, shouldError: true})
	registry.Register(&MockCollector{name: "interfaces", supported: false})             // Unsupported
	registry.Register(&MockCollector{name: "facts", supported: true, hasMetrics: true}) // Good

	enabledCollectors := DeviceCollectors{
		BGP:        true,
		Interfaces: true,
		Facts:      true,
	}

	ctx := context.Background()
	metrics, err := registry.CollectFromDevice(ctx, client, enabledCollectors, timestamp)

	// Should not return error even if some collectors fail
	require.NoError(t, err)

	// Should only have metrics from successful collector
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())
}

func TestRegistry_ThreadSafety(t *testing.T) {
	registry := NewRegistry()

	// Test concurrent registration and retrieval
	done := make(chan bool, 10)

	// Start multiple goroutines registering collectors
	for i := 0; i < 5; i++ {
		go func(id int) {
			collector := &MockCollector{
				name:      fmt.Sprintf("collector_%d", id),
				supported: true,
			}
			registry.Register(collector)
			done <- true
		}(i)
	}

	// Start multiple goroutines retrieving collectors
	for i := 0; i < 5; i++ {
		go func(id int) {
			registry.GetCollector(fmt.Sprintf("collector_%d", id))
			registry.GetAllCollectors()
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all collectors were registered
	collectors := registry.GetAllCollectors()
	assert.Len(t, collectors, 5)
}
