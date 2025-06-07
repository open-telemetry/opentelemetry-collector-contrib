// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

// compressData in gzip format
func compressData(tb testing.TB, buf []byte) []byte {
	var compressedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedData)
	_, err := gzipWriter.Write(buf)
	require.NoError(tb, err)
	err = gzipWriter.Close()
	require.NoError(tb, err)
	return compressedData.Bytes()
}

// getLogFromFile reads the data inside the file and returns
// it in the format the data: each line is a json log, and
// the final data is gzip compressed.
func getLogFromFile(t *testing.T, dir string, file string) []byte {
	data, err := os.ReadFile(filepath.Join(dir, file))
	require.NoError(t, err)
	compacted := bytes.NewBuffer([]byte{})
	err = gojson.Compact(compacted, data)
	require.NoError(t, err)
	return compressData(t, compacted.Bytes())
}

func TestUnmarshalLogs(t *testing.T) {
	t.Parallel()

	dir := "testdata"
	tests := map[string]struct {
		record           []byte
		expectedFilename string
		expectedErr      string
	}{
		"valid_log": {
			record:           getLogFromFile(t, dir, "valid_log.json"),
			expectedFilename: "valid_log_expected.yaml",
		},
		"invalid_gzip": {
			record:      []byte("invalid"),
			expectedErr: "failed to decompress content",
		},
		"invalid_json": {
			record:      compressData(t, []byte("invalid")),
			expectedErr: "failed to unmarshal WAF log",
		},
	}

	u := wafLogUnmarshaler{buildInfo: component.BuildInfo{}}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logs, err := u.UnmarshalLogs(test.record)
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

func TestSetKeyAttributes(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		webACLID   string
		key        resourceKey
		expectsErr string
	}{
		"valid": {
			webACLID: "arn:aws:wafv2:us-east-1:1234:global/webacl/test-waf/e3132a63",
			key: resourceKey{
				region:     "us-east-1",
				accountID:  "1234",
				resourceID: "arn:aws:wafv2:us-east-1:1234:global/webacl/test-waf/e3132a63",
			},
		},
		"unexpected_prefix": {
			webACLID:   "invalid",
			expectsErr: "does not have expected prefix",
		},
		"no_region": {
			webACLID:   "arn:aws:wafv2::",
			expectsErr: "could not find region",
		},
		"no_account": {
			webACLID:   "arn:aws:wafv2:us-east-1::",
			expectsErr: "could not find account",
		},
		"invalid_format": {
			webACLID:   "arn:aws:wafv2:us-east-1:1234:",
			expectsErr: "does not have expected format",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := &resourceKey{}
			err := setKeyAttributes(test.webACLID, res)

			if test.expectsErr != "" {
				require.ErrorContains(t, err, test.expectsErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.key, *res)
		})
	}
}
