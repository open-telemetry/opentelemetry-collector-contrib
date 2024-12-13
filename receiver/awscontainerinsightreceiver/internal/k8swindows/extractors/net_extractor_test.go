// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package extractors

import (
	"testing"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	cExtractor "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetStats(t *testing.T) {
	result := testutils.LoadKubeletSummary(t, "./testdata/PreSingleKubeletSummary.json")
	result2 := testutils.LoadKubeletSummary(t, "./testdata/CurSingleKubeletSummary.json")

	nodeRawMetric := ConvertNodeToRaw(result.Node)
	nodeRawMetric2 := ConvertNodeToRaw(result2.Node)

	containerType := ci.TypeNode
	extractor := NewNetMetricExtractor(nil)
	var cMetrics []*stores.CIMetricImpl
	if extractor.HasValue(nodeRawMetric) {
		cMetrics = extractor.GetValue(nodeRawMetric, nil, containerType)
	}
	if extractor.HasValue(nodeRawMetric2) {
		cMetrics = extractor.GetValue(nodeRawMetric2, nil, containerType)
	}

	expectedFields := []map[string]any{
		{
			"node_interface_network_rx_bytes":    float64(3474.633333333333),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_total_bytes": float64(5477.549999999999),
			"node_interface_network_tx_bytes":    float64(2002.9166666666665),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
		},
		{
			"node_interface_network_rx_bytes":    float64(2293.733333333333),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_total_bytes": float64(4781.45),
			"node_interface_network_tx_bytes":    float64(2487.7166666666667),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
		},
		{
			"node_interface_network_rx_bytes":    float64(0),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_total_bytes": float64(0),
			"node_interface_network_tx_bytes":    float64(0),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
		},
		{
			"node_interface_network_rx_bytes":    float64(0),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_total_bytes": float64(0),
			"node_interface_network_tx_bytes":    float64(0),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
		},
		{
			"node_interface_network_rx_bytes":    float64(0),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_total_bytes": float64(0),
			"node_interface_network_tx_bytes":    float64(0),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
		},
		{
			"node_interface_network_rx_bytes":    float64(0),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_total_bytes": float64(0),
			"node_interface_network_tx_bytes":    float64(0),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
		},
		{
			"node_interface_network_rx_bytes":    float64(0),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_total_bytes": float64(0),
			"node_interface_network_tx_bytes":    float64(0),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
		},
		{
			"node_interface_network_rx_bytes":    float64(0),
			"node_interface_network_rx_dropped":  float64(0),
			"node_interface_network_rx_errors":   float64(0),
			"node_interface_network_total_bytes": float64(0),
			"node_interface_network_tx_bytes":    float64(0),
			"node_interface_network_tx_dropped":  float64(0),
			"node_interface_network_tx_errors":   float64(0),
		},
		{
			"node_network_rx_bytes":    float64(5768.366666666667),
			"node_network_rx_dropped":  float64(0),
			"node_network_rx_errors":   float64(0),
			"node_network_total_bytes": float64(10259),
			"node_network_tx_bytes":    float64(4490.633333333333),
			"node_network_tx_dropped":  float64(0),
			"node_network_tx_errors":   float64(0),
		},
	}

	expectedTags := []map[string]string{
		{
			"Type":      "NodeNet",
			"interface": "Amazon Elastic Network Adapter",
		},
		{
			"Type":      "NodeNet",
			"interface": "Hyper-V Virtual Ethernet Adapter",
		},
		{
			"Type":      "NodeNet",
			"interface": "Teredo Tunneling Pseudo-Interface",
		},
		{
			"Type":      "NodeNet",
			"interface": "AWS PV Network Device",
		},
		{
			"Type":      "NodeNet",
			"interface": "Microsoft IP-HTTPS Platform Interface",
		},
		{
			"Type":      "NodeNet",
			"interface": "Hyper-V Virtual Switch Extension Adapter",
		},
		{
			"Type":      "NodeNet",
			"interface": "Microsoft Kernel Debug Network Adapter",
		},
		{
			"Type":      "NodeNet",
			"interface": "6to4 Adapter",
		},
		{
			"Type": "Node",
		},
	}

	assert.Equal(t, len(cMetrics), 9)
	for i := range expectedFields {
		cExtractor.AssertContainsTaggedField(t, cMetrics[i], expectedFields[i], expectedTags[i])
	}
	require.NoError(t, extractor.Shutdown())

	// pod type metrics
	podRawMetric := ConvertPodToRaw(result.Pods[0])
	podRawMetric2 := ConvertPodToRaw(result2.Pods[0])

	containerType = ci.TypePod
	extractor = NewNetMetricExtractor(nil)

	if extractor.HasValue(podRawMetric) {
		cMetrics = extractor.GetValue(podRawMetric, nil, containerType)
	}
	if extractor.HasValue(podRawMetric2) {
		cMetrics = extractor.GetValue(podRawMetric2, nil, containerType)
	}

	expectedFields = []map[string]any{
		{
			"pod_interface_network_rx_bytes":    float64(1735.9333333333334),
			"pod_interface_network_rx_dropped":  float64(0),
			"pod_interface_network_rx_errors":   float64(0),
			"pod_interface_network_total_bytes": float64(1903.75),
			"pod_interface_network_tx_bytes":    float64(167.81666666666666),
			"pod_interface_network_tx_dropped":  float64(0),
			"pod_interface_network_tx_errors":   float64(0),
		},

		{
			"pod_network_rx_bytes":    float64(1735.9333333333334),
			"pod_network_rx_dropped":  float64(0),
			"pod_network_rx_errors":   float64(0),
			"pod_network_total_bytes": float64(1903.75),
			"pod_network_tx_bytes":    float64(167.81666666666666),
			"pod_network_tx_dropped":  float64(0),
			"pod_network_tx_errors":   float64(0),
		},
	}

	expectedTags = []map[string]string{
		{
			"Type":      "PodNet",
			"interface": "cid-ccecfbf3-5ecf-4d93-9229-3f2f0df1b6b5",
		},
		{
			"Type": "Pod",
		},
	}

	assert.Equal(t, len(cMetrics), 2)
	for i := range expectedFields {
		cExtractor.AssertContainsTaggedField(t, cMetrics[i], expectedFields[i], expectedTags[i])
	}
	require.NoError(t, extractor.Shutdown())
}
