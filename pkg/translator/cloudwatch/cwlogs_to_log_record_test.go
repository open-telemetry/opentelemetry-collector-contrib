package cloudwatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/pdata/plog"
	"os"
	"path/filepath"
	"testing"
)

func TestUnmarshalLogs(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		filename string
		err      error
	}{
		"SingleRecord": {
			filename: "single_record",
		},
		"MultipleRecords": {
			filename: "multiple_records",
		},
		"InvalidRecord": {
			filename: "invalid_record",
			err: fmt.Errorf(
				"cloudwatch log from datum [0] is invalid: %w",
				errors.New("cloudwatch log is missing timestamp field"),
			),
		},
		"SomeInvalidRecord": {
			filename: "some_invalid_record",
			err: fmt.Errorf(
				"cloudwatch log from datum [1] is invalid: %w",
				errors.New("cloudwatch log is missing timestamp field"),
			),
		},
		"EmptyRecord": {
			filename: "empty_record",
			err:      errors.New("no log records could be obtained from the record"),
		},
	}
	unmarshaller := &plog.JSONUnmarshaler{}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata/log", testCase.filename+".json"))
			require.NoError(t, err)
			// since new line represents the end of the record, we
			// need to remove all new lines from the json file, so
			// the record will not get incorrectly split. We keep
			// the new lines between different records.
			var buf bytes.Buffer
			gjson.ParseBytes(content).ForEach(func(_, value gjson.Result) bool {
				err := json.NewEncoder(&buf).Encode(value.Value())
				require.NoError(t, err)
				return true
			})
			result, err := UnmarshalLogs(buf.Bytes())
			require.Equal(t, testCase.err, err)
			if err != nil {
				return
			}

			content, err = os.ReadFile(filepath.Join("testdata/log", testCase.filename+"_expected.json"))
			require.NoError(t, err)
			expected, err := unmarshaller.UnmarshalLogs(content)
			require.NoError(t, err)

			// get log records
			expectedLogs := expected.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
			require.Equal(t, expectedLogs, result)
		})
	}
}
