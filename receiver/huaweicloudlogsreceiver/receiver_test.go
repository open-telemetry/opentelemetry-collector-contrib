// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudlogsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudlogsreceiver"

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/lts/v2/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudlogsreceiver/internal/mocks"
)

func stringPtr(s string) *string {
	return &s
}

func TestNewReceiver(t *testing.T) {
	cfg := &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Second,
		},
	}
	mr := newHuaweiCloudLogsReceiver(receivertest.NewNopSettings(), cfg, new(consumertest.LogsSink))
	assert.NotNil(t, mr)
}

func TestListLogsSuccess(t *testing.T) {
	mc := mocks.NewLtsClient(t)

	mc.On("ListLogs", mock.Anything).Return(&model.ListLogsResponse{
		Logs: &[]model.LogContents{
			{
				Content: stringPtr("2020-07-25/14:40:43 this <HighLightTag>log</HighLightTag> is Error NO 2"),
				LineNum: stringPtr("10"),
				Labels: map[string]string{
					"hostName":      "ecs-kwxtest",
					"hostIP":        "192.168.0.156",
					"appName":       "default_appname",
					"containerName": "CONFIG_FILE",
					"clusterName":   "CONFIG_FILE",
					"hostId":        "9787ef31-fd7b-4eff-ba71-72d580f11f55",
					"podName":       "default_procname",
					"clusterId":     "CONFIG_FILE",
					"nameSpace":     "CONFIG_FILE",
					"category":      "LTS",
				},
			},
			{
				Content: stringPtr("2020-07-26/15:00:43 this <HighLightTag>log</HighLightTag> is Error NO 3"),
			},
		},
	}, nil)

	receiver := &logsReceiver{
		client: mc,
		config: createDefaultConfig().(*Config),
	}

	logs, err := receiver.listLogs(context.Background(), time.Now(), time.Now())

	assert.NoError(t, err)
	assert.Len(t, logs, 2)
	mc.AssertExpectations(t)
}

func TestListLogsFailure(t *testing.T) {
	mc := mocks.NewLtsClient(t)

	mc.On("ListLogs", mock.Anything).Return(nil, errors.New("failed to list logs"))
	receiver := &logsReceiver{
		client: mc,
		config: createDefaultConfig().(*Config),
	}

	logs, err := receiver.listLogs(context.Background(), time.Now(), time.Now())

	assert.Error(t, err)
	assert.Empty(t, logs)
	assert.Equal(t, "failed to list logs", err.Error())
	mc.AssertExpectations(t)
}

func TestPollLogsAndConsumeSuccess(t *testing.T) {
	mc := mocks.NewLtsClient(t)
	next := new(consumertest.LogsSink)
	receiver := newHuaweiCloudLogsReceiver(receivertest.NewNopSettings(), &Config{}, next)
	receiver.client = mc

	mc.On("ListLogs", mock.Anything).Return(&model.ListLogsResponse{
		Logs: &[]model.LogContents{
			{
				Content: stringPtr("2020-07-25/14:40:43 this <HighLightTag>log</HighLightTag> is Error NO 2"),
			},
			{
				Content: stringPtr("2020-07-26/15:00:43 this <HighLightTag>log</HighLightTag> is Error NO 3"),
			},
		},
	}, nil)

	err := receiver.pollLogsAndConsume(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, next.LogRecordCount())
}
