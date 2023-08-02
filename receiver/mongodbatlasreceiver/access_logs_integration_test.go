// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package mongodbatlasreceiver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/metadata"
)

func TestAccessLogsIntegration(t *testing.T) {
	mockClient := mockAccessLogsClient{}

	payloadFile, err := os.ReadFile(filepath.Join("testdata", "accesslogs", "sample-payloads", "sample-access-logs.json"))
	require.NoError(t, err)

	var accessLogs []*mongodbatlas.AccessLogs
	err = json.Unmarshal(payloadFile, &accessLogs)
	require.NoError(t, err)

	mockClient.On("GetProject", mock.Anything, testProjectName).Return(&mongodbatlas.Project{
		ID:    testProjectID,
		Name:  testProjectName,
		OrgID: testOrgID,
	}, nil)
	mockClient.On("GetClusters", mock.Anything, testProjectID).Return(
		[]mongodbatlas.Cluster{
			{
				GroupID: testProjectID,
				Name:    testClusterName,
			},
		},
		nil)
	mockClient.On("GetAccessLogs", mock.Anything, testProjectID, testClusterName, mock.Anything).Return(accessLogs, nil)

	sink := &consumertest.LogsSink{}
	fact := NewFactory()

	recv, err := fact.CreateLogsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		&Config{
			ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			Logs: LogConfig{
				Enabled: true,
				Projects: []*LogsProjectConfig{
					{
						ProjectConfig: ProjectConfig{
							Name: testProjectName,
						},
						AccessLogs: &AccessLogsConfig{
							PollInterval: 1 * time.Second,
						},
					},
				},
			},
		},
		sink,
	)
	require.NoError(t, err)

	rcvr, ok := recv.(*combinedLogsReceiver)
	require.True(t, ok)
	rcvr.accessLogs.client = &mockClient

	err = recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 5*time.Second, 10*time.Millisecond)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()[0]
	expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", "accesslogs", "golden", "retrieved-logs.yaml"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreObservedTimestamp()))
}
