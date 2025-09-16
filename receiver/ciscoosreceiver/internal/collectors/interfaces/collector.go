// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfaces // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/interfaces"

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/rpc"
)

// Collector implements the Interfaces collector for Cisco devices
type Collector struct {
	parser        *Parser
	metricBuilder *collectors.MetricBuilder
}

// NewCollector creates a new Interfaces collector
func NewCollector() *Collector {
	return &Collector{
		parser:        NewParser(),
		metricBuilder: collectors.NewMetricBuilder(),
	}
}

// Name returns the collector name
func (c *Collector) Name() string {
	return "interfaces"
}

// IsSupported checks if Interfaces collection is supported on the device
func (c *Collector) IsSupported(client *rpc.Client) bool {
	return client.IsOSSupported("interfaces")
}

// Collect performs Interfaces metric collection from the device
func (c *Collector) Collect(ctx context.Context, client *rpc.Client, timestamp time.Time) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()

	// Get Interfaces command for this OS type
	command := client.GetCommand("interfaces")
	if command == "" {
		return metrics, fmt.Errorf("Interfaces command not supported on OS type: %s", client.GetOSType())
	}

	// Execute interfaces command
	output, err := client.ExecuteCommand(command)
	if err != nil {
		// Try fallback command if primary fails
		fallbackCommand := "show interface brief"
		output, err = client.ExecuteCommand(fallbackCommand)
		if err != nil {
			return metrics, fmt.Errorf("failed to execute Interfaces commands '%s' and '%s': %w", command, fallbackCommand, err)
		}
	}

	// Parse interfaces
	interfaces, err := c.parser.ParseInterfaces(output)
	if err != nil {
		return metrics, fmt.Errorf("failed to parse Interfaces output: %w", err)
	}

	// If no interfaces found with main parser, try simple parsing
	if len(interfaces) == 0 {
		interfaces, err = c.parser.ParseSimpleInterfaces(output)
		if err != nil {
			return metrics, fmt.Errorf("failed to parse simple interfaces: %w", err)
		}
	}

	// Try to get VLAN information (IOS XE specific)
	if client.GetOSType() == rpc.IOSXE {
		vlanCommand := client.GetCommand("interfaces_vlans")
		if vlanCommand != "" {
			vlanOutput, err := client.ExecuteCommand(vlanCommand)
			if err == nil {
				c.parser.ParseVLANs(vlanOutput, interfaces)
			}
		}
	}

	// Generate metrics for each interface
	target := client.GetTarget()

	// Security fix: Extract only host part, exclude port for metrics
	host := target
	if strings.Contains(target, ":") {
		d := strings.Split(target, ":")
		host = d[0] // Extract only the IP part
		// port := d[1] // Port available if needed but not used in metrics
	}

	for _, iface := range interfaces {
		c.generateInterfaceMetrics(metrics, iface, host, timestamp)
	}

	return metrics, nil
}

// generateInterfaceMetrics creates OpenTelemetry metrics for a network interface
// Only generates cisco_exporter-compatible metrics (11 total)
func (c *Collector) generateInterfaceMetrics(metrics pmetric.Metrics, iface *Interface, target string, timestamp time.Time) {
	// Common attributes for all interface metrics (matching cisco_exporter labels)
	baseAttributes := map[string]string{
		"target": target,
		"name":   iface.Name,
	}

	// Add optional attributes if available
	if iface.Description != "" {
		baseAttributes["description"] = iface.Description
	}
	if iface.MACAddress != "" {
		baseAttributes["mac"] = iface.MACAddress
	}
	if iface.Speed > 0 {
		if iface.SpeedString != "" {
			baseAttributes["speed"] = iface.SpeedString // e.g., "1000 Mb/s"
		} else {
			baseAttributes["speed"] = fmt.Sprintf("%d", iface.Speed)
		}
	}

	// 1. cisco_interface_admin_up - Administrative status (1 = up, 0 = down)
	adminStatus := iface.GetAdminStatusInt()
	c.metricBuilder.CreateGaugeMetric(
		metrics,
		internal.MetricPrefix+"interface_admin_up",
		"Interface administrative status (1 = up, 0 = down)",
		"1",
		adminStatus,
		timestamp,
		baseAttributes,
	)

	// 2. cisco_interface_up - Operational status (1 = up, 0 = down)
	operStatus := iface.GetOperStatusInt()
	c.metricBuilder.CreateGaugeMetric(
		metrics,
		internal.MetricPrefix+"interface_up",
		"Interface operational status (1 = up, 0 = down)",
		"1",
		operStatus,
		timestamp,
		baseAttributes,
	)

	// 3. cisco_interface_error_status - Error status (1 = errors present, 0 = no errors)
	c.metricBuilder.CreateGaugeMetric(
		metrics,
		internal.MetricPrefix+"interface_error_status",
		"Interface error status (1 = errors present, 0 = no errors)",
		"1",
		iface.GetErrorStatusInt(),
		timestamp,
		baseAttributes,
	)

	// 4. cisco_interface_receive_bytes - Bytes received on interface
	c.metricBuilder.CreateGaugeMetricFloat64(
		metrics,
		internal.MetricPrefix+"interface_receive_bytes",
		"Bytes received on interface",
		"bytes",
		iface.InputBytes,
		timestamp,
		baseAttributes,
	)

	// 5. cisco_interface_transmit_bytes - Bytes transmitted on interface
	c.metricBuilder.CreateGaugeMetricFloat64(
		metrics,
		internal.MetricPrefix+"interface_transmit_bytes",
		"Bytes transmitted on interface",
		"bytes",
		iface.OutputBytes,
		timestamp,
		baseAttributes,
	)

	// 6. cisco_interface_receive_errors - Number of errors caused by incoming packets
	c.metricBuilder.CreateGaugeMetricFloat64(
		metrics,
		internal.MetricPrefix+"interface_receive_errors",
		"Number of errors caused by incoming packets",
		"1",
		iface.InputErrors,
		timestamp,
		baseAttributes,
	)

	// 7. cisco_interface_transmit_errors - Number of errors caused by outgoing packets
	c.metricBuilder.CreateGaugeMetricFloat64(
		metrics,
		internal.MetricPrefix+"interface_transmit_errors",
		"Number of errors caused by outgoing packets",
		"1",
		iface.OutputErrors,
		timestamp,
		baseAttributes,
	)

	// 8. cisco_interface_receive_drops - Number of dropped incoming packets
	c.metricBuilder.CreateGaugeMetricFloat64(
		metrics,
		internal.MetricPrefix+"interface_receive_drops",
		"Number of dropped incoming packets",
		"1",
		iface.InputDrops,
		timestamp,
		baseAttributes,
	)

	// 9. cisco_interface_transmit_drops - Number of dropped outgoing packets
	c.metricBuilder.CreateGaugeMetricFloat64(
		metrics,
		internal.MetricPrefix+"interface_transmit_drops",
		"Number of dropped outgoing packets",
		"1",
		iface.OutputDrops,
		timestamp,
		baseAttributes,
	)

	// 10. cisco_interface_receive_broadcast - Received broadcast packets
	c.metricBuilder.CreateGaugeMetricFloat64(
		metrics,
		internal.MetricPrefix+"interface_receive_broadcast",
		"Received broadcast packets",
		"1",
		iface.InputBroadcast,
		timestamp,
		baseAttributes,
	)

	// 11. cisco_interface_receive_multicast - Received multicast packets
	c.metricBuilder.CreateGaugeMetricFloat64(
		metrics,
		internal.MetricPrefix+"interface_receive_multicast",
		"Received multicast packets",
		"1",
		iface.InputMulticast,
		timestamp,
		baseAttributes,
	)
}

// GetMetricNames returns the names of metrics this collector generates
func (c *Collector) GetMetricNames() []string {
	return []string{
		// cisco_exporter compatible metrics only (11 total)
		internal.MetricPrefix + "interface_admin_up",
		internal.MetricPrefix + "interface_up",
		internal.MetricPrefix + "interface_error_status",
		internal.MetricPrefix + "interface_receive_bytes",
		internal.MetricPrefix + "interface_transmit_bytes",
		internal.MetricPrefix + "interface_receive_errors",
		internal.MetricPrefix + "interface_transmit_errors",
		internal.MetricPrefix + "interface_receive_drops",
		internal.MetricPrefix + "interface_transmit_drops",
		internal.MetricPrefix + "interface_receive_broadcast",
		internal.MetricPrefix + "interface_receive_multicast",
	}
}

// GetRequiredCommands returns the commands this collector needs to execute
func (c *Collector) GetRequiredCommands() []string {
	return []string{
		"show interfaces",
		"show vlans", // Optional, for IOS XE
	}
}
