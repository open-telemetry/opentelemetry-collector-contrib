// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/rpc"
)

// Collector defines the interface that all Cisco collectors must implement
type Collector interface {
	// Name returns the collector name (bgp, environment, facts, interfaces, optics)
	Name() string

	// Collect performs metric collection from the device
	Collect(ctx context.Context, client *rpc.Client, timestamp time.Time) (pmetric.Metrics, error)

	// IsSupported checks if this collector is supported on the device OS
	IsSupported(client *rpc.Client) bool
}

// CollectorConfig holds configuration for a specific collector
type CollectorConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// DeviceCollectors holds the configuration for all collectors on a device
type DeviceCollectors struct {
	BGP         bool `mapstructure:"bgp"`
	Environment bool `mapstructure:"environment"`
	Facts       bool `mapstructure:"facts"`
	Interfaces  bool `mapstructure:"interfaces"`
	Optics      bool `mapstructure:"optics"`
}

// IsEnabled checks if a specific collector is enabled
func (dc *DeviceCollectors) IsEnabled(collectorName string) bool {
	switch collectorName {
	case "bgp":
		return dc.BGP
	case "environment":
		return dc.Environment
	case "facts":
		return dc.Facts
	case "interfaces":
		return dc.Interfaces
	case "optics":
		return dc.Optics
	default:
		return false
	}
}

// MetricBuilder provides helper methods for building OpenTelemetry metrics
type MetricBuilder struct{}

// NewMetricBuilder creates a new metric builder
func NewMetricBuilder() *MetricBuilder {
	return &MetricBuilder{}
}

// CreateGaugeMetric creates a gauge metric with the specified parameters
func (mb *MetricBuilder) CreateGaugeMetric(
	metrics pmetric.Metrics,
	name, description, unit string,
	value int64,
	timestamp time.Time,
	attributes map[string]string,
) {
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	resource := resourceMetrics.Resource()

	// Add resource attributes
	resource.Attributes().PutStr("service.name", "ciscoosreceiver")
	resource.Attributes().PutStr("service.version", "1.0.0")

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scope := scopeMetrics.Scope()
	scope.SetName("ciscoosreceiver")
	scope.SetVersion("1.0.0")

	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(name)
	metric.SetDescription(description)
	metric.SetUnit(unit)

	gauge := metric.SetEmptyGauge()
	dataPoint := gauge.DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	dataPoint.SetIntValue(value)

	// Add attributes
	for key, val := range attributes {
		dataPoint.Attributes().PutStr(key, val)
	}
}

// CreateGaugeMetricFloat64 creates a gauge metric with float64 value
func (mb *MetricBuilder) CreateGaugeMetricFloat64(
	metrics pmetric.Metrics,
	name, description, unit string,
	value float64,
	timestamp time.Time,
	attributes map[string]string,
) {
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	resource := resourceMetrics.Resource()

	// Add resource attributes
	resource.Attributes().PutStr("service.name", "ciscoosreceiver")
	resource.Attributes().PutStr("service.version", "1.0.0")

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scope := scopeMetrics.Scope()
	scope.SetName("ciscoosreceiver")
	scope.SetVersion("1.0.0")

	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(name)
	metric.SetDescription(description)
	metric.SetUnit(unit)

	gauge := metric.SetEmptyGauge()
	dataPoint := gauge.DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	dataPoint.SetDoubleValue(value)

	// Add attributes
	for key, val := range attributes {
		dataPoint.Attributes().PutStr(key, val)
	}
}

// CreateCounterMetric creates a counter metric with the specified parameters
func (mb *MetricBuilder) CreateCounterMetric(
	metrics pmetric.Metrics,
	name, description, unit string,
	value int64,
	timestamp time.Time,
	attributes map[string]string,
) {
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	resource := resourceMetrics.Resource()

	// Add resource attributes
	resource.Attributes().PutStr("service.name", "ciscoosreceiver")
	resource.Attributes().PutStr("service.version", "1.0.0")

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scope := scopeMetrics.Scope()
	scope.SetName("ciscoosreceiver")
	scope.SetVersion("1.0.0")

	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(name)
	metric.SetDescription(description)
	metric.SetUnit(unit)

	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(true)
	dataPoint := sum.DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	dataPoint.SetIntValue(value)

	// Add attributes
	for key, val := range attributes {
		dataPoint.Attributes().PutStr(key, val)
	}
}
