package cloudwatch

import (
	"bytes"
	"compress/gzip"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"testing"
)

func TestUnmarshalLogs(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		filename      string
		resourceCount int
		logCount      int
		err           error
	}{
		"WithMultipleRecords": {
			filename:      "multiple_records",
			resourceCount: 1,
			logCount:      2,
		},
		"WithSingleRecord": {
			filename:      "single_record",
			resourceCount: 1,
			logCount:      1,
		},
		"WithInvalidRecords": {
			filename: "invalid_records",
			err:      errInvalidLogRecord,
		},
		"WithSomeInvalidRecords": {
			filename:      "some_invalid_records",
			resourceCount: 1,
			logCount:      2,
		},
		"WithMultipleResources": {
			filename:      "multiple_resources",
			resourceCount: 3,
			logCount:      6,
		},
	}
	for _, test := range tests {
		t.Run(test.filename, func(t *testing.T) {
			record, err := os.ReadFile(filepath.Join("testdata/log", test.filename))
			require.NoError(t, err)

			var buf bytes.Buffer
			g := gzip.NewWriter(&buf)
			_, err = g.Write(record)
			require.NoError(t, err)
			err = g.Close()
			require.NoError(t, err)
			compressedRecord := buf.Bytes()

			logs, err := UnmarshalLogs(compressedRecord, zap.NewNop())
			require.Equal(t, test.err, err)
			require.Equal(t, test.resourceCount, logs.ResourceLogs().Len())
			require.Equal(t, test.logCount, logs.LogRecordCount())
		})
	}
}
