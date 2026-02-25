// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestVSANMetricsFromMetadata ensures hasEnabledVSANMetrics() covers all vSAN metrics
// defined in metadata.yaml. This test will fail if a new vSAN metric is added to
// metadata.yaml but not included in hasEnabledVSANMetrics().
func TestVSANMetricsFromMetadata(t *testing.T) {
	vSANMetricsFromYAML := getVSANMetricsFromMetadataYAML(t)
	checkedMetrics := getCheckedVSANMetrics()

	// Convert to sets for comparison
	yamlSet := make(map[string]bool)
	for _, metric := range vSANMetricsFromYAML {
		yamlSet[metric] = true
	}

	checkedSet := make(map[string]bool)
	for _, metric := range checkedMetrics {
		checkedSet[metric] = true
	}

	var missingInFunction []string
	for metric := range yamlSet {
		if !checkedSet[metric] {
			missingInFunction = append(missingInFunction, metric)
		}
	}

	var extraInFunction []string
	for metric := range checkedSet {
		if !yamlSet[metric] {
			extraInFunction = append(extraInFunction, metric)
		}
	}

	if len(missingInFunction) > 0 {
		t.Errorf("hasEnabledVSANMetrics() is missing these vSAN metrics from metadata.yaml:")
		for _, metric := range missingInFunction {
			t.Errorf("   - %s", metric)
		}
		t.Error("Please add the missing metrics to hasEnabledVSANMetrics() function")
	}

	if len(extraInFunction) > 0 {
		t.Errorf("hasEnabledVSANMetrics() checks these metrics not found in metadata.yaml:")
		for _, metric := range extraInFunction {
			t.Errorf("   - %s", metric)
		}
		t.Error("Please remove non-vSAN metrics from hasEnabledVSANMetrics() function")
	}

	if len(missingInFunction) == 0 && len(extraInFunction) == 0 {
		t.Logf("All %d vSAN metrics from metadata.yaml are properly checked", len(vSANMetricsFromYAML))
	}
}

type metadataYAML struct {
	Metrics map[string]any `yaml:"metrics"`
}

func getVSANMetricsFromMetadataYAML(t *testing.T) []string {
	metadataPath := findMetadataYAML(t)

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata.yaml: %v", err)
	}

	var metadata metadataYAML
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("Failed to parse metadata.yaml: %v", err)
	}

	// Extract vSAN metrics
	var vSANMetrics []string
	for metricName := range metadata.Metrics {
		if strings.Contains(metricName, "vsan") {
			vSANMetrics = append(vSANMetrics, metricName)
		}
	}

	return vSANMetrics
}

func findMetadataYAML(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Look for metadata.yaml in current directory
	metadataPath := filepath.Join(dir, "metadata.yaml")
	if _, err := os.Stat(metadataPath); err == nil {
		return metadataPath
	}

	t.Fatalf("Could not find metadata.yaml")
	return ""
}

// getCheckedVSANMetrics returns the metrics checked in hasEnabledVSANMetrics()
func getCheckedVSANMetrics() []string {
	// This should match exactly what's in hasEnabledVSANMetrics()
	return []string{
		// Cluster vSAN metrics
		"vcenter.cluster.vsan.congestions",
		"vcenter.cluster.vsan.latency.avg",
		"vcenter.cluster.vsan.operations",
		"vcenter.cluster.vsan.throughput",

		// Host vSAN metrics
		"vcenter.host.vsan.cache.hit_rate",
		"vcenter.host.vsan.congestions",
		"vcenter.host.vsan.latency.avg",
		"vcenter.host.vsan.operations",
		"vcenter.host.vsan.throughput",

		// VM vSAN metrics
		"vcenter.vm.vsan.latency.avg",
		"vcenter.vm.vsan.operations",
		"vcenter.vm.vsan.throughput",
	}
}
