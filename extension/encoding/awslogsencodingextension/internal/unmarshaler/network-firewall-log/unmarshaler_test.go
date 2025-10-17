// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package networkfirewall

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

// compressToGZIPReader compresses buf into a gzip-formatted io.Reader.
func compressToGZIPReader(t *testing.T, buf []byte) io.Reader {
	var compressedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedData)
	_, err := gzipWriter.Write(buf)
	require.NoError(t, err)
	err = gzipWriter.Close()
	require.NoError(t, err)
	gzipReader, err := gzip.NewReader(bytes.NewReader(compressedData.Bytes()))
	require.NoError(t, err)
	return gzipReader
}

// readAndCompressLogFile reads the data inside it, compacts the JSON
// struct, compresses it and returns a GZIP reader for it.
func readAndCompressLogFile(t *testing.T, dir, file string) io.Reader {
	data, err := os.ReadFile(filepath.Join(dir, file))
	require.NoError(t, err)
	compacted := bytes.NewBuffer([]byte{})
	err = gojson.Compact(compacted, data)
	require.NoError(t, err)
	return compressToGZIPReader(t, compacted.Bytes())
}

func TestUnmarshalLogs(t *testing.T) {
	t.Parallel()

	dir := "testdata"
	tests := map[string]struct {
		reader           io.Reader
		expectedFilename string
		expectedErr      string
	}{
		"valid_alert_log": {
			reader:           readAndCompressLogFile(t, dir, "valid_alert_log.json"),
			expectedFilename: "valid_alert_log_expected.yaml",
		},
		"valid_flow_log": {
			reader:           readAndCompressLogFile(t, dir, "valid_flow_log.json"),
			expectedFilename: "valid_flow_log_expected.yaml",
		},
		"valid_tls_log": {
			reader:           readAndCompressLogFile(t, dir, "valid_tls_log.json"),
			expectedFilename: "valid_tls_log_expected.yaml",
		},
		"missing_firewall_name": {
			reader:      readAndCompressLogFile(t, dir, "missing_firewall_name.json"),
			expectedErr: "invalid Network Firewall log: empty firewall_name field",
		},
		"invalid_json": {
			reader:      compressToGZIPReader(t, []byte("invalid")),
			expectedErr: "failed to unmarshal Network Firewall log",
		},
	}

	u := networkFirewallLogUnmarshaler{buildInfo: component.BuildInfo{}}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logs, err := u.UnmarshalAWSLogs(test.reader)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
				return
			}

			require.NoError(t, err)

			expected, err := golden.ReadLogs(filepath.Join(dir, test.expectedFilename))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expected, logs))
		})
	}
}

func TestSetResourceAttributes(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		firewallName     string
		availabilityZone string
		expectsErr       string
		checkFunc        func(t *testing.T, attrs map[string]any)
	}{
		"valid_with_az": {
			firewallName:     "test-firewall",
			availabilityZone: "us-east-1a",
			checkFunc: func(t *testing.T, attrs map[string]any) {
				require.Equal(t, "test-firewall", attrs["aws.networkfirewall.name"])
				require.Equal(t, "us-east-1a", attrs["cloud.availability_zone"])
			},
		},
		"valid_without_az": {
			firewallName:     "test-firewall",
			availabilityZone: "",
			checkFunc: func(t *testing.T, attrs map[string]any) {
				require.Equal(t, "test-firewall", attrs["aws.networkfirewall.name"])
				require.NotContains(t, attrs, "cloud.availability_zone")
			},
		},
		"missing_firewall_name": {
			firewallName: "",
			expectsErr:   "firewall_name is required",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logs := plog.NewLogs()
			rl := logs.ResourceLogs().AppendEmpty()
			err := setResourceAttributes(rl, test.firewallName, test.availabilityZone)

			if test.expectsErr != "" {
				require.ErrorContains(t, err, test.expectsErr)
				return
			}

			require.NoError(t, err)
			if test.checkFunc != nil {
				test.checkFunc(t, rl.Resource().Attributes().AsRaw())
			}
		})
	}
}
