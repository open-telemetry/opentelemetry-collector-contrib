// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"
)

func TestNetStats(t *testing.T) {
	result := testutils.LoadContainerInfo(t, "./testdata/PreInfoNode.json")
	result2 := testutils.LoadContainerInfo(t, "./testdata/CurInfoNode.json")

	containerType := ci.TypeNode
	extractor := NewNetMetricExtractor(nil)
	var cMetrics []*CAdvisorMetric
	if extractor.HasValue(result[0]) {
		cMetrics = extractor.GetValue(result[0], nil, containerType)
	}
	if extractor.HasValue(result2[0]) {
		cMetrics = extractor.GetValue(result2[0], nil, containerType)
	}

	expectedFields := []map[string]any{
		{
			"node_interface_network_rx_bytes":    float64(382.28706877648807),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_rx_packets":  float64(2.2498322426788007),
			"node_interface_network_total_bytes": float64(2644.3827413026775),
			"node_interface_network_tx_bytes":    float64(2262.0956725261894),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
			"node_interface_network_tx_packets":  float64(2.2867147384604203),
		},
		{
			"node_interface_network_rx_bytes":    float64(0),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_rx_packets":  float64(0),
			"node_interface_network_total_bytes": float64(0),
			"node_interface_network_tx_bytes":    float64(0),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
			"node_interface_network_tx_packets":  float64(0),
		},
		{
			"node_interface_network_rx_bytes":    float64(265.24046841351793),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_rx_packets":  float64(2.397362225805279),
			"node_interface_network_total_bytes": float64(4050.730746703727),
			"node_interface_network_tx_bytes":    float64(3785.490278290209),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
			"node_interface_network_tx_packets":  float64(2.5264509610409482),
		},
		{
			"node_interface_network_rx_bytes":    float64(7818.037954573597),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_rx_packets":  float64(19.197339054333046),
			"node_interface_network_total_bytes": float64(10891.751388022218),
			"node_interface_network_tx_bytes":    float64(3073.713433448621),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
			"node_interface_network_tx_packets":  float64(18.21995291612012),
		},
		{
			"node_interface_network_rx_bytes":    float64(319.3286484772632),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_rx_packets":  float64(3.632925834489539),
			"node_interface_network_total_bytes": float64(1910.8452239499343),
			"node_interface_network_tx_bytes":    float64(1591.516575472671),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
			"node_interface_network_tx_packets":  float64(3.651367082380349),
		},
		{
			"node_interface_network_rx_bytes":    float64(1616.061876415339),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_rx_packets":  float64(1.862566036971794),
			"node_interface_network_total_bytes": float64(1748.9863912122962),
			"node_interface_network_tx_bytes":    float64(132.92451479695734),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
			"node_interface_network_tx_packets":  float64(1.7150360538453153),
		},
		{
			"node_interface_network_rx_bytes":    float64(268.76274676066265),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_rx_packets":  float64(2.6370984483858075),
			"node_interface_network_total_bytes": float64(1131.8131480505633),
			"node_interface_network_tx_bytes":    float64(863.0504012899006),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
			"node_interface_network_tx_packets":  float64(2.6370984483858075),
		},
		{
			"node_network_rx_bytes":    float64(10669.718763416868),
			"node_network_rx_dropped":  float64(0),
			"node_network_rx_errors":   float64(0),
			"node_network_rx_packets":  float64(31.977123842664266),
			"node_network_total_bytes": float64(22378.509639241416),
			"node_network_tx_bytes":    float64(11708.790875824548),
			"node_network_tx_dropped":  float64(0),
			"node_network_tx_errors":   float64(0),
			"node_network_tx_packets":  float64(31.03662020023296),
		},
	}

	expectedTags := []map[string]string{
		{
			"Type":      "NodeNet",
			"interface": "eni2bbf9bbc6ab",
		},
		{
			"Type":      "NodeNet",
			"interface": "eni5b727305f03",
		},
		{
			"Type":      "NodeNet",
			"interface": "enic36ed3f6bb5",
		},
		{
			"Type":      "NodeNet",
			"interface": "eth0",
		},
		{
			"Type":      "NodeNet",
			"interface": "eni40c8ef3f6c3",
		},
		{
			"Type":      "NodeNet",
			"interface": "eth1",
		},
		{
			"Type":      "NodeNet",
			"interface": "eni7cce1b61ea4",
		},
		{
			"Type": "Node",
		},
	}

	assert.Equal(t, len(cMetrics), 8)
	for i := range expectedFields {
		AssertContainsTaggedField(t, cMetrics[i], expectedFields[i], expectedTags[i])
	}
	require.NoError(t, extractor.Shutdown())
}
