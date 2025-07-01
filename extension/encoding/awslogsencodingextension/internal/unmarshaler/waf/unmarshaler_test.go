// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package waf

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
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"

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
func readAndCompressLogFile(t *testing.T, dir string, file string) io.Reader {
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
		"valid_log": {
			reader:           readAndCompressLogFile(t, dir, "valid_log.json"),
			expectedFilename: "valid_log_expected.yaml",
		},
		"missing_web_acl_id": {
			reader:      readAndCompressLogFile(t, dir, "missing_webaclid_log.json"),
			expectedErr: "invalid WAF log: empty webaclId field",
		},
		"invalid_json": {
			reader:      compressToGZIPReader(t, []byte("invalid")),
			expectedErr: "failed to unmarshal WAF log",
		},
	}

	u := wafLogUnmarshaler{buildInfo: component.BuildInfo{}}
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

func TestSetKeyAttributes(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		webACLID   string
		expectsMap map[string]any
		expectsErr string
	}{
		"valid": {
			webACLID: "arn:aws:wafv2:us-east-1:1234:global/webacl/test-waf/e3132a63",
			expectsMap: map[string]any{
				string(conventions.CloudRegionKey):     "us-east-1",
				string(conventions.CloudAccountIDKey):  "1234",
				string(conventions.CloudResourceIDKey): "arn:aws:wafv2:us-east-1:1234:global/webacl/test-waf/e3132a63",
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
			rl := plog.NewResourceLogs()
			err := setResourceAttributes(rl, test.webACLID)

			if test.expectsErr != "" {
				require.ErrorContains(t, err, test.expectsErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expectsMap, rl.Resource().Attributes().AsRaw())
		})
	}
}
