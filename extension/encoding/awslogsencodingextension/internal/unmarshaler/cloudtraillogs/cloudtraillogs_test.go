// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudtraillogs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

const (
	filesDirectory = "testdata"
)

func TestCloudTrailLogsUnmarshaler_Unmarshal_CreateUser(t *testing.T) {
	unmarshaler := NewCloudTrailLogsUnmarshaler(component.BuildInfo{})
	data, err := os.ReadFile(filepath.Join(filesDirectory, "cloudtrail_createUser_logs.json"))
	require.NoError(t, err)

	logs, err := unmarshaler.UnmarshalLogs(data)
	require.NoError(t, err)

	expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, "cloudtrail_createUser_logs_expected.yaml"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expectedLogs, logs))
}

func TestCloudTrailLogsUnmarshaler_Unmarshal_StartInstances(t *testing.T) {
	unmarshaler := NewCloudTrailLogsUnmarshaler(component.BuildInfo{})
	data, err := os.ReadFile(filepath.Join(filesDirectory, "cloudtrail_startInstances_log.json"))
	require.NoError(t, err)

	logs, err := unmarshaler.UnmarshalLogs(data)
	require.NoError(t, err)

	expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, "cloudtrail_startInstances_log_expected.yaml"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expectedLogs, logs))
}
