package waf

import (
	"bytes"
	"compress/gzip"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"os"
	"path/filepath"
	"testing"
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

// getLogFromFile reads the data inside
// the file and returns it in the format the data
// is expected to be: gzip compressed.
func getLogFromFile(t *testing.T, dir string, file string) []byte {
	data, err := os.ReadFile(filepath.Join(dir, file))
	require.NoError(t, err)
	return compressData(t, data)
}

func TestUnmarshalLogs(t *testing.T) {
	t.Parallel()

	dir := "testdata"
	tests := map[string]struct {
		record           []byte
		expectedFilename string
		expectedErr      string
	}{
		"valid_s3_access_log": {
			// Same access log as in https://docs.aws.amazon.com/AmazonS3/latest/userguide/LogFormat.html
			record:           getLogFromFile(t, dir, "valid_log_1.json"),
			expectedFilename: "valid_log_1_expected.yaml",
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

			golden.WriteLogs(t, filepath.Join(dir, test.expectedFilename), logs)

			expected, err := golden.ReadLogs(filepath.Join(dir, test.expectedFilename))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expected, logs))
		})
	}
}
