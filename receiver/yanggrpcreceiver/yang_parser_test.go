// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal"
)

func TestYANGParser(t *testing.T) {
	parser := internal.NewYANGParser()
	parser.LoadBuiltinModules()

	t.Run("TestBuiltinModulesLoaded", func(t *testing.T) {
		modules := parser.GetAvailableModules()
		assert.Contains(t, modules, "Cisco-IOS-XE-interfaces-oper")
		assert.Contains(t, modules, "Cisco-IOS-XE-bgp-oper")
		assert.Contains(t, modules, "Cisco-IOS-XE-ospf-oper")
	})

	t.Run("TestInterfaceKeyDetection", func(t *testing.T) {
		key := parser.GetKeyForPath("Cisco-IOS-XE-interfaces-oper", "/interfaces/interface")
		assert.Equal(t, "name", key, "Should identify 'name' as the key for interface list")
	})

	t.Run("TestPathAnalysis", func(t *testing.T) {
		encodingPath := "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics"

		analysis := parser.AnalyzeEncodingPath(encodingPath)
		require.NotNil(t, analysis, "Analysis should not be nil")

		assert.Equal(t, "Cisco-IOS-XE-interfaces-oper", analysis.ModuleName)
		assert.Equal(t, "/interfaces/interface", analysis.ListPath)
		assert.Equal(t, "name", analysis.Keys["/interfaces/interface"])

		t.Logf("Analysis result: %+v", analysis)
	})

	t.Run("TestKeyFieldIdentification", func(t *testing.T) {
		encodingPath := "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics"
		analysis := parser.AnalyzeEncodingPath(encodingPath)

		// Create a mock grpc service to test key identification
		service := &grpcService{yangParser: parser}

		// Test that "name" is identified as a key field
		isKey := service.isKeyField("name", analysis)
		assert.True(t, isKey, "Field 'name' should be identified as a key field")

		// Test that regular statistics fields are not key fields
		isKey = service.isKeyField("in-octets", analysis)
		assert.False(t, isKey, "Field 'in-octets' should not be identified as a key field")
	})

	t.Run("TestBGPPathAnalysis", func(t *testing.T) {
		encodingPath := "Cisco-IOS-XE-bgp-oper:bgp-state-data/neighbors/neighbor"

		analysis := parser.AnalyzeEncodingPath(encodingPath)
		require.NotNil(t, analysis)

		assert.Equal(t, "Cisco-IOS-XE-bgp-oper", analysis.ModuleName)
		t.Logf("BGP Analysis: %+v", analysis)
	})
}

func TestYANGParserIntegration(t *testing.T) {
	parser := internal.NewYANGParser()
	parser.LoadBuiltinModules()

	// Test real telemetry scenarios
	testCases := []struct {
		name         string
		encodingPath string
		expectedKey  string
	}{
		{
			name:         "Interface Statistics",
			encodingPath: "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics",
			expectedKey:  "name",
		},
		{
			name:         "BGP Neighbor",
			encodingPath: "Cisco-IOS-XE-bgp-oper:bgp-state-data/neighbors/neighbor",
			expectedKey:  "neighbor-id",
		},
		{
			name:         "OSPF Instance",
			encodingPath: "Cisco-IOS-XE-ospf-oper:ospf-oper-data/ospf-state/ospf-instance",
			expectedKey:  "router-id",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			analysis := parser.AnalyzeEncodingPath(tc.encodingPath)
			require.NotNil(t, analysis, "Analysis should not be nil for %s", tc.name)

			assert.Equal(t, tc.expectedKey, analysis.Keys[analysis.ListPath],
				"Expected key %s for %s", tc.expectedKey, tc.name)
		})
	}
}

func TestYANGParserPersistence(t *testing.T) {
	parser := internal.NewYANGParser()
	parser.LoadBuiltinModules()

	// Test saving modules to file
	t.Run("SaveToFile", func(t *testing.T) {
		filename := filepath.Join(t.TempDir(), "yang_modules_test.json")
		err := parser.SaveModulesToFile(filename)
		assert.NoError(t, err)

		// Test loading from file
		parser2 := internal.NewYANGParser()
		err = parser2.LoadModulesFromFile(filename)
		assert.NoError(t, err)

		// Verify loaded modules match
		originalModules := parser.GetAvailableModules()
		loadedModules := parser2.GetAvailableModules()
		assert.ElementsMatch(t, originalModules, loadedModules)
	})
}

func TestFieldNameExtraction(t *testing.T) {
	service := &grpcService{}

	testCases := []struct {
		input    string
		expected string
	}{
		{"content.name", "name"},
		{"keys.interface-name", "interface-name"},
		{"statistics.in-octets", "in-octets"},
		{"content.discontinuity-time_info", "discontinuity-time"},
		{"simple", "simple"},
	}

	for _, tc := range testCases {
		result := service.extractFieldName(tc.input)
		assert.Equal(t, tc.expected, result, "Expected %s but got %s for input %s", tc.expected, result, tc.input)
	}
}

// TestYANGEnhancedTelemetryProcessing tests the integration with real telemetry data
func TestYANGEnhancedTelemetryProcessing(t *testing.T) {
	// This test would simulate the enhanced telemetry processing with YANG awareness
	parser := internal.NewYANGParser()
	parser.LoadBuiltinModules()

	// Test path analysis for live telemetry
	liveEncodingPath := "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics"
	analysis := parser.AnalyzeEncodingPath(liveEncodingPath)

	require.NotNil(t, analysis)
	assert.Equal(t, "Cisco-IOS-XE-interfaces-oper", analysis.ModuleName)
	assert.Equal(t, "/interfaces/interface", analysis.ListPath)
	assert.Contains(t, analysis.Keys, "/interfaces/interface")
	assert.Equal(t, "name", analysis.Keys["/interfaces/interface"])

	// Test that this matches the actual telemetry we're receiving
	t.Logf("YANG Analysis for live telemetry:")
	t.Logf("  Module: %s", analysis.ModuleName)
	t.Logf("  List Path: %s", analysis.ListPath)
	t.Logf("  Keys: %+v", analysis.Keys)

	// Serialize analysis for inspection
	analysisJSON, err := json.MarshalIndent(analysis, "", "  ")
	require.NoError(t, err)
	t.Logf("Analysis JSON:\n%s", analysisJSON)
}
