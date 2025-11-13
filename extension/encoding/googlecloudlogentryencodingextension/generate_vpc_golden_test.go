//go:build generate_vpc_goldens

package googlecloudlogentryencodingextension

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
)

// TestGenerateVPCGoldens refreshes the YAML goldens used by the integration
// tests that exercise VPC flow parsing. It is guarded by the
// `generate_vpc_goldens` build tag so it only runs when explicitly invoked,
// e.g. `go test -tags generate_vpc_goldens ./extension/... -run TestGenerateVPCGoldens`.
// The fixtures are stored as newline-delimited JSON logs; the helper rewrites
// each one to match the encoder output.
//
// Note that the NDJSON files those are based on the output of the export_vpc_flow_logs.sh script.
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
			jsonFile: "vpc-flow-log-google-service.ndjson",
			outFile:  "vpc-flow-log-google-service_expected.yaml",
		},
		{
			name:     "managed_instance",
			jsonFile: "vpc-flow-log-managed-instance.ndjson",
			outFile:  "vpc-flow-log-managed-instance_expected.yaml",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join(root, tc.jsonFile))
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			lines := bytes.Split(raw, []byte{'\n'})
			var buf bytes.Buffer
			for _, line := range lines {
				line = bytes.TrimSpace(line)
				if len(line) == 0 {
					continue
				}
				buf.Write(line)
				buf.WriteByte('\n')
			}
			data := bytes.TrimSpace(buf.Bytes())

			logs, err := ext.UnmarshalLogs(data)
			if err != nil {
				t.Fatalf("failed to unmarshal logs: %v", err)
			}

			if err := golden.WriteLogsToFile(filepath.Join(root, tc.outFile), logs); err != nil {
				t.Fatalf("failed to write golden file: %v", err)
			}
		})
	}
}
