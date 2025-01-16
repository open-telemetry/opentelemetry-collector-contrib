// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestUnmarshalMetrics(t *testing.T) {
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
		"MultipleResources": {
			filename: "multiple_resources",
		},
		"InvalidRecord": {
			filename: "invalid_record",
			err: fmt.Errorf(
				"cloudwatch metric from datum [0] is not valid: %w",
				errors.New("cloudwatch metric is missing metric name field"),
			),
		},
		"SomeInvalidRecord": {
			filename: "some_invalid_record",
			err: fmt.Errorf(
				"cloudwatch metric from datum [1] is not valid: %w",
				errors.New("cloudwatch metric is missing metric name field"),
			),
		},
		"EmptyRecord": {
			filename: "empty_record",
			err:      errors.New("no resource metrics could be obtained from the record"),
		},
	}
	unmarshaller := &pmetric.JSONUnmarshaler{}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata/metric", testCase.filename+".json"))
			require.NoError(t, err)

			var buf bytes.Buffer
			// the file has a list of metrics
			// we want to remove the list bytes [ ]
			gjson.ParseBytes(content).ForEach(func(_, value gjson.Result) bool {
				err := json.NewEncoder(&buf).Encode(value.Value())
				require.NoError(t, err)
				return true
			})
			result, err := UnmarshalMetrics(buf.Bytes())
			require.Equal(t, testCase.err, err)
			if err != nil {
				return
			}

			content, err = os.ReadFile(filepath.Join("testdata/metric", testCase.filename+"_expected.json"))
			require.NoError(t, err)
			expected, err := unmarshaller.UnmarshalMetrics(content)
			require.NoError(t, err)

			require.Equal(t, expected, result)
		})
	}
}
