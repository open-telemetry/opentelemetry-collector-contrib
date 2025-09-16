// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bgp // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/bgp"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/rpc"
)

// Collector implements the BGP collector for Cisco devices
type Collector struct {
	parser        *Parser
	metricBuilder *collectors.MetricBuilder
}

// NewCollector creates a new BGP collector
func NewCollector() *Collector {
	return &Collector{
		parser:        NewParser(),
		metricBuilder: collectors.NewMetricBuilder(),
	}
}

// Name returns the collector name
func (c *Collector) Name() string {
	return "bgp"
}

// IsSupported checks if BGP collection is supported on the device
func (c *Collector) IsSupported(client *rpc.Client) bool {
	return client.IsOSSupported("bgp")
}

// Collect performs BGP metric collection from the device
func (c *Collector) Collect(ctx context.Context, client *rpc.Client, timestamp time.Time) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()

	command := client.GetCommand("bgp")
	if command == "" {
		return metrics, fmt.Errorf("BGP command not supported on OS type: %s", client.GetOSType())
	}
	output, err := client.ExecuteCommand(command)
	if err != nil {
		fallbackCommands := []string{"show bgp summary", "show ip bgp", "show bgp ipv4 unicast summary"}
		for _, fallbackCmd := range fallbackCommands {
			output, err = client.ExecuteCommand(fallbackCmd)
			if err == nil {
				break
			}
		}
		if err != nil {
			return metrics, fmt.Errorf("failed to execute BGP command '%s': %w", command, err)
		}
	}

	if !c.parser.ValidateOutput(output) {
		return metrics, fmt.Errorf("invalid BGP output received")
	}
	sessions, err := c.parser.ParseBGPSummary(output)
	if err != nil {
		return metrics, fmt.Errorf("failed to parse BGP output: %w", err)
	}

	target := client.GetTarget()
	for _, session := range sessions {
		attributes := map[string]string{
			"target": target,
			"asn":    session.ASN,
			"ip":     session.NeighborIP,
		}

		c.metricBuilder.CreateGaugeMetric(
			metrics,
			internal.MetricPrefix+"bgp_session_up",
			"Session is up (1 = Established)",
			"",
			session.GetUpStatus(),
			timestamp,
			attributes,
		)

		c.metricBuilder.CreateGaugeMetric(
			metrics,
			internal.MetricPrefix+"bgp_session_prefixes_received_count",
			"Number of received prefixes",
			"",
			session.PrefixesReceived,
			timestamp,
			attributes,
		)

		c.metricBuilder.CreateGaugeMetric(
			metrics,
			internal.MetricPrefix+"bgp_session_messages_input_count",
			"Number of received messages",
			"",
			session.MessagesInput,
			timestamp,
			attributes,
		)

		c.metricBuilder.CreateGaugeMetric(
			metrics,
			internal.MetricPrefix+"bgp_session_messages_output_count",
			"Number of transmitted messages",
			"",
			session.MessagesOutput,
			timestamp,
			attributes,
		)
	}

	return metrics, nil
}

// GetMetricNames returns the names of metrics this collector generates
func (c *Collector) GetMetricNames() []string {
	return []string{
		internal.MetricPrefix + "bgp_session_up",
		internal.MetricPrefix + "bgp_session_prefixes_received_count",
		internal.MetricPrefix + "bgp_session_messages_input_count",
		internal.MetricPrefix + "bgp_session_messages_output_count",
	}
}

// GetRequiredCommands returns the commands this collector needs to execute
func (c *Collector) GetRequiredCommands() []string {
	return []string{
		"show bgp all summary",
	}
}
