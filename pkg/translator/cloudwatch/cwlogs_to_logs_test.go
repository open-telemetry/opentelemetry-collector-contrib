// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestUnmarshalLogs(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		filename string
		err      error
	}{
		"ValidCloudwatchLog": {
			filename: "valid_log",
		},
		"InvalidCloudwatchLog": {
			filename: "invalid",
			err: fmt.Errorf(
				"cloudwatch log is invalid: %w",
				errors.New("cloudwatch log is missing timestamp field"),
			),
		},
	}
	unmarshaller := &plog.JSONUnmarshaler{}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata", testCase.filename+".json"))
			require.NoError(t, err)

			result, err := UnmarshalLogs(content)
			require.Equal(t, testCase.err, err)
			if err != nil {
				return
			}

			content, err = os.ReadFile(filepath.Join("testdata", testCase.filename+"_expected.json"))
			require.NoError(t, err)
			expected, err := unmarshaller.UnmarshalLogs(content)

			require.NoError(t, err)
			require.Equal(t, expected, result)
		})
	}
}
