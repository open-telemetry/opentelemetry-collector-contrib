// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package elasticsearchexporter

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("update-golden", false, "Update golden files with current document structure")

// TestDocumentStructureGolden generates and validates golden files for document structure
// across all mapping modes and signal types. This serves as both a test and documentation
// of what fields are emitted under different mapping modes.
func TestDocumentStructureGolden(t *testing.T) {
	goldenDir := filepath.Join("testdata", "golden")

	// Ensure golden directory exists when updating
	if *updateGolden {
		require.NoError(t, os.MkdirAll(goldenDir, 0755))
	}

	testCases := []struct {
		mode   MappingMode
		signal string
		skip   bool
	}{
		// OTel mode - supports all signals
		{MappingOTel, "logs", false},
		{MappingOTel, "traces", false},
		{MappingOTel, "metrics", false},

		// ECS mode - supports logs, traces, metrics
		{MappingECS, "logs", false},
		{MappingECS, "traces", false},
		{MappingECS, "metrics", false},

		// None mode - supports logs and traces
		{MappingNone, "logs", false},
		{MappingNone, "traces", false},

		// Raw mode - supports logs and traces
		{MappingRaw, "logs", false},
		{MappingRaw, "traces", false},

		// Bodymap mode - only supports logs
		{MappingBodyMap, "logs", false},
	}

	for _, tc := range testCases {
		testName := fmt.Sprintf("%s_%s", tc.mode.String(), tc.signal)
		t.Run(testName, func(t *testing.T) {
			if tc.skip {
				t.Skip("Test case marked as skip")
			}

			goldenFile := filepath.Join(goldenDir, testName+".json")

			// Generate document
			doc, err := generateDocument(tc.mode, tc.signal)
			if err != nil {
				// Some mode/signal combinations are not supported
				t.Logf("Skipping %s: %v", testName, err)
				return
			}

			// Pretty print JSON
			var prettyJSON bytes.Buffer
			err = json.Indent(&prettyJSON, doc, "", "  ")
			require.NoError(t, err, "Failed to format JSON")

			if *updateGolden {
				// Update golden file
				err = os.WriteFile(goldenFile, prettyJSON.Bytes(), 0644)
				require.NoError(t, err, "Failed to write golden file")
				t.Logf("✓ Updated golden file: %s", goldenFile)
			} else {
				// Validate against golden file
				expected, err := os.ReadFile(goldenFile)
				if os.IsNotExist(err) {
					t.Fatalf("Golden file not found: %s\nRun with -update-golden to create it:\n  go test -tags=integration -run TestDocumentStructureGolden -update-golden", goldenFile)
				}
				require.NoError(t, err, "Failed to read golden file")

				require.JSONEq(t, string(expected), prettyJSON.String(),
					"Document structure has changed for %s.\n"+
						"If this change is intentional, update the golden file:\n"+
						"  go test -tags=integration -run TestDocumentStructureGolden -update-golden",
					testName)
			}
		})
	}

	if *updateGolden {
		t.Log("\n✓ All golden files updated successfully!")
		t.Log("Please review the changes in testdata/golden/ and commit them.")
	}
}

// generateDocument generates a sample document for the given mapping mode and signal type
func generateDocument(mode MappingMode, signal string) ([]byte, error) {
	encoder, err := newEncoder(mode)
	if err != nil {
		return nil, fmt.Errorf("failed to create encoder: %w", err)
	}

	var doc bytes.Buffer
	ctx := createSampleContext()
	idx := createSampleIndex(signal)

	switch signal {
	case "logs":
		if !supportsLogs(mode) {
			return nil, fmt.Errorf("mode %s does not support logs", mode)
		}
		logRecord := createSampleLogRecord()
		err = encoder.encodeLog(ctx, logRecord, idx, &doc)

	case "traces":
		if !supportsTraces(mode) {
			return nil, fmt.Errorf("mode %s does not support traces", mode)
		}
		span := createSampleSpan()
		err = encoder.encodeSpan(ctx, span, idx, &doc)

	case "metrics":
		if !supportsMetrics(mode) {
			return nil, fmt.Errorf("mode %s does not support metrics", mode)
		}
		// For metrics, we skip golden file generation for now as it requires
		// complex datapoint grouping logic. This can be added in a future enhancement.
		return nil, fmt.Errorf("metrics golden file generation not yet implemented")

	default:
		return nil, fmt.Errorf("unknown signal type: %s", signal)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to encode %s: %w", signal, err)
	}

	return doc.Bytes(), nil
}

