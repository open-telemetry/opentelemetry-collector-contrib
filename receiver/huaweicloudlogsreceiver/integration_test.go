// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudlogsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudlogsreceiver"

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/lts/v2/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudlogsreceiver/internal/mocks"
)

func TestHuaweiCloudLogsReceiverIntegration(t *testing.T) {
	mc := mocks.NewLtsClient(t)

	mc.On("ListLogs", mock.Anything).Return(&model.ListLogsResponse{
		Logs: &[]model.LogContents{
			{
				Content: stringPtr("2020-07-25/14:40:00 this log is Error NO 2"),
				LineNum: stringPtr("123"),
				Labels: map[string]string{
					"hostName":      "ecs-test1",
					"hostIP":        "192.168.0.106",
					"containerName": "CONFIG_FILE",
					"category":      "LTS",
				},
			},
			{
				Content: stringPtr("2020-07-25/14:50:00 this log is Error NO 3"),
				LineNum: stringPtr("456"),
				Labels: map[string]string{
					"hostName":      "ecs-test2",
					"hostIP":        "192.168.0.206",
					"containerName": "CONFIG_FILE",
					"category":      "LTS",
				},
			},
		},
	}, nil)

	sink := &consumertest.LogsSink{}
	cfg := createDefaultConfig().(*Config)
	cfg.RegionID = "us-east-2"
	cfg.CollectionInterval = time.Second
	cfg.ProjectID = "my-project"
	cfg.GroupID = "group-1"
	cfg.StreamID = "stream-1"

	recv, err := NewFactory().CreateLogsReceiver(
		context.Background(),
		receivertest.NewNopSettings(),
		cfg,
		sink,
	)
	require.NoError(t, err)

	rcvr, ok := recv.(*logsReceiver)
	require.True(t, ok)
	rcvr.client = mc

	err = recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 5*time.Second, 10*time.Millisecond)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()[0]

	expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", "golden", "logs_golden.yaml"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreLogRecordsOrder()))
}
