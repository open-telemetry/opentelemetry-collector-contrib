// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal"
)

func TestYANGParser(t *testing.T) {
	parser := internal.NewYANGParser(zap.NewNop())
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

	t.Run("TestBGPPathAnalysis", func(t *testing.T) {
		encodingPath := "Cisco-IOS-XE-bgp-oper:bgp-state-data/neighbors/neighbor"

		analysis := parser.AnalyzeEncodingPath(encodingPath)
		require.NotNil(t, analysis)

		assert.Equal(t, "Cisco-IOS-XE-bgp-oper", analysis.ModuleName)
		t.Logf("BGP Analysis: %+v", analysis)
	})
}

func TestYANGParserIntegration(t *testing.T) {
	parser := internal.NewYANGParser(zap.NewNop())
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
	parser := internal.NewYANGParser(zap.NewNop())
	parser.LoadBuiltinModules()

	// Test saving modules to file
	t.Run("SaveToFile", func(t *testing.T) {
		filename := filepath.Join(t.TempDir(), "yang_modules_test.json")
		err := parser.SaveModulesToFile(filename)
		assert.NoError(t, err)

		// Test loading from file
		parser2 := internal.NewYANGParser(zap.NewNop())
		err = parser2.LoadModulesFromFile(filename)
		assert.NoError(t, err)

		// Verify loaded modules match
		originalModules := parser.GetAvailableModules()
		loadedModules := parser2.GetAvailableModules()
		assert.ElementsMatch(t, originalModules, loadedModules)
	})
}

// TestYANGEnhancedTelemetryProcessing tests the integration with real telemetry data
func TestYANGEnhancedTelemetryProcessing(t *testing.T) {
	// This test would simulate the enhanced telemetry processing with YANG awareness
	parser := internal.NewYANGParser(zap.NewNop())
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

func TestYANGParser_DuplicateModuleDetection(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	logger := zap.New(core)
	parser := internal.NewYANGParser(logger)

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	yang1 := `
module test-dup {
    namespace "urn:test";
    prefix "td";

    list item {
        key "k1";
        leaf k1 {
            type string;
        }
    }
}
`
	yang2 := `
module test-dup {
    namespace "urn:test";
    prefix "td";

    list item {
        key "k2";
        leaf k2 {
            type string;
        }
    }
}
`

	f1 := filepath.Join(dir1, "dup.yang")
	f2 := filepath.Join(dir2, "dup.yang")
	require.NoError(t, os.WriteFile(f1, []byte(yang1), 0o600))
	require.NoError(t, os.WriteFile(f2, []byte(yang2), 0o600))

	require.NoError(t, parser.ExtractYANGFromFiles(dir1))
	assert.Equal(t, 0, recorded.Len())
	assert.Equal(t, "k1", parser.GetKeyForPath("test-dup", "/item"))

	require.NoError(t, parser.ExtractYANGFromFiles(dir2))
	require.Equal(t, 1, recorded.Len())

	warn := recorded.All()[0]
	assert.Equal(t, "Duplicate YANG module detected; overwriting existing module", warn.Message)
	ctxMap := warn.ContextMap()
	assert.Equal(t, "test-dup", ctxMap["module"])
	assert.Equal(t, f1, ctxMap["previous_path"])
	assert.Equal(t, f2, ctxMap["new_path"])
	assert.Equal(t, f2, ctxMap["winner"])

	assert.Equal(t, "k2", parser.GetKeyForPath("test-dup", "/item"))
	assert.Equal(t, f2, parser.GetModuleSource("test-dup"))
}
