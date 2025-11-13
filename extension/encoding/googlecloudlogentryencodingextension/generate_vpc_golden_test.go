// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build generate_vpc_goldens

package googlecloudlogentryencodingextension

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
)

// TestGenerateVPCGoldens refreshes the YAML goldens used by the integration
// tests that exercise VPC flow parsing. It is guarded by the
// `generate_vpc_goldens` build tag so it only runs when explicitly invoked,
// e.g. `go test -tags generate_vpc_goldens ./extension/... -run TestGenerateVPCGoldens`.
// The fixtures are stored as multi-line JSON logs; the helper compacts and rewrites
// each one to match the encoder output.
//
// Note that the JSON files those are based on the output of the export_vpc_flow_logs.sh script.
// See scripts/README.md in this package for more details.
func TestGenerateVPCGoldens(t *testing.T) {
	t.Parallel()

	cfg := createDefaultConfig().(*Config)
	ext := newExtension(cfg)

	root := "testdata/vpc-flow-log"
	testCases := []struct {
		name     string
		jsonFile string
		outFile  string
	}{
		{
			name:     "google_service",
			jsonFile: "vpc-flow-log-google-service.json",
			outFile:  "vpc-flow-log-google-service_expected.yaml",
		},
		{
			name:     "managed_instance",
			jsonFile: "vpc-flow-log-managed-instance.json",
			outFile:  "vpc-flow-log-managed-instance_expected.yaml",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Join(root, tc.jsonFile))
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			// Compact multi-line JSON to a single line
			content := bytes.NewBuffer(nil)
			if err := gojson.Compact(content, data); err != nil {
				t.Fatalf("failed to compact JSON: %v", err)
			}

			logs, err := ext.UnmarshalLogs(content.Bytes())
			if err != nil {
				t.Fatalf("failed to unmarshal logs: %v", err)
			}

			if err := golden.WriteLogsToFile(filepath.Join(root, tc.outFile), logs); err != nil {
				t.Fatalf("failed to write golden file: %v", err)
			}
		})
	}
}
