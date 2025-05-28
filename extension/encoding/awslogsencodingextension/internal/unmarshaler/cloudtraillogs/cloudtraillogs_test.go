// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudtraillogs

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

const (
	filesDirectory = "testdata"
)

func compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)

	_, err := gw.Write(data)
	if err != nil {
		return nil, err
	}

	if err := gw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func TestCloudTrailLogsUnmarshaler_Unmarshal_MultipleRecords_Compressed(t *testing.T) {
	unmarshaler := NewCloudTrailLogsUnmarshaler(component.BuildInfo{})
	data, err := os.ReadFile(filepath.Join(filesDirectory, "cloudtrail_logs.json"))
	require.NoError(t, err)

	compressedData, err := compressData(data)
	require.NoError(t, err)

	actualLogs, err := unmarshaler.UnmarshalLogs(compressedData)
	require.NoError(t, err)

	expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, "cloudtrail_logs_expected.yaml"))
	require.NoError(t, err)

	compareLogsIgnoringResourceOrder(t, expectedLogs, actualLogs)
}

func compareLogsIgnoringResourceOrder(t *testing.T, expected, actual plog.Logs) {
	require.Positive(t, actual.ResourceLogs().Len(), "No resource logs found in actual logs")

	err := plogtest.CompareLogs(expected, actual,
		plogtest.IgnoreResourceLogsOrder(),
		plogtest.IgnoreScopeLogsOrder(),
		plogtest.IgnoreLogRecordsOrder())
	require.NoError(t, err, "Logs don't match")
}
