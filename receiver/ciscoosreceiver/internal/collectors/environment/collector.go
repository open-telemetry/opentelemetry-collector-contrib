// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package environment // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/environment"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/rpc"
)

// Collector implements the Environment collector for Cisco devices
type Collector struct {
	parser        *Parser
	metricBuilder *collectors.MetricBuilder
}

// NewCollector creates a new Environment collector
func NewCollector() *Collector {
	return &Collector{
		parser:        NewParser(),
		metricBuilder: collectors.NewMetricBuilder(),
	}
}

// Name returns the collector name
func (c *Collector) Name() string {
	return "environment"
}

// IsSupported checks if Environment collection is supported on the device
func (c *Collector) IsSupported(client *rpc.Client) bool {
	return client.IsOSSupported("environment")
}

// Collect performs Environment metric collection from the device
func (c *Collector) Collect(ctx context.Context, client *rpc.Client, timestamp time.Time) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()

	// Get Environment command for this OS type
	command := client.GetCommand("environment")
	if command == "" {
		return metrics, fmt.Errorf("Environment command not supported on OS type: %s", client.GetOSType())
	}

	// Execute environment command
	output, err := client.ExecuteCommand(command)
	if err != nil {
		return metrics, fmt.Errorf("failed to execute Environment command '%s': %w", command, err)
	}

	// Parse environment sensors
	sensors, err := c.parser.ParseEnvironment(output)
	if err != nil {
		return metrics, fmt.Errorf("failed to parse Environment output: %w", err)
	}

	// If no sensors found with main parser, try simple temperature parsing
	if len(sensors) == 0 {
		sensors, err = c.parser.ParseSimpleTemperature(output)
		if err != nil {
			return metrics, fmt.Errorf("failed to parse simple temperature: %w", err)
		}
	}

	// Generate metrics for each sensor
	target := client.GetTarget()
	for _, sensor := range sensors {
		c.generateSensorMetrics(metrics, sensor, target, timestamp)
	}

	return metrics, nil
}

// generateSensorMetrics creates OpenTelemetry metrics for an environmental sensor
// Only generates cisco_exporter-compatible metrics (2 total)
func (c *Collector) generateSensorMetrics(metrics pmetric.Metrics, sensor *Sensor, target string, timestamp time.Time) {
	// Common attributes for cisco_exporter compatibility (matching cisco_exporter labels)
	baseAttributes := map[string]string{
		"target": target,
		"item":   sensor.Name,
	}

	// Add status attribute if available
	if sensor.Status != "" {
		baseAttributes["status"] = sensor.Status
	}

	// 1. cisco_environment_sensor_temp - Temperature readings
	if sensor.IsTemperature() {
		c.metricBuilder.CreateGaugeMetric(
			metrics,
			internal.MetricPrefix+"environment_sensor_temp",
			"Temperature sensor readings",
			"celsius",
			sensor.GetNumericValue(),
			timestamp,
			baseAttributes,
		)
	}

	// 2. cisco_environment_power_up - Power supply status (1=OK, 0=Problem)
	if sensor.IsPowerSupply() {
		powerUpValue := int64(0)
		if sensor.IsHealthy {
			powerUpValue = 1
		}

		c.metricBuilder.CreateGaugeMetric(
			metrics,
			internal.MetricPrefix+"environment_power_up",
			"Status of power supplies (1 OK, 0 Something is wrong)",
			"1",
			powerUpValue,
			timestamp,
			baseAttributes,
		)
	}
}

// GetMetricNames returns the names of metrics this collector generates
func (c *Collector) GetMetricNames() []string {
	return []string{
		internal.MetricPrefix + "environment_sensor_temp",
		internal.MetricPrefix + "environment_power_up",
	}
}

// GetRequiredCommands returns the commands this collector needs to execute
func (c *Collector) GetRequiredCommands() []string {
	return []string{
		"show environment",
	}
}
