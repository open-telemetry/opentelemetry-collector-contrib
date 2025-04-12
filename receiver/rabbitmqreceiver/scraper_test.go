// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/models"
)

func TestScraperStart(t *testing.T) {
	clientConfigNonexistentCA := confighttp.NewDefaultClientConfig()
	clientConfigNonexistentCA.Endpoint = defaultEndpoint
	clientConfigNonexistentCA.TLSSetting = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "/non/existent",
		},
	}

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint

	testcases := []struct {
		desc        string
		scraper     *rabbitmqScraper
		expectError bool
	}{
		{
			desc: "Bad Config",
			scraper: &rabbitmqScraper{
				cfg: &Config{
					ClientConfig: clientConfigNonexistentCA,
				},
				settings: componenttest.NewNopTelemetrySettings(),
			},
			expectError: true,
		},
		{
			desc: "Valid Config",
			scraper: &rabbitmqScraper{
				cfg: &Config{
					ClientConfig: clientConfig,
				},
				settings: componenttest.NewNopTelemetrySettings(),
			},
			expectError: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.scraper.start(context.Background(), componenttest.NewNopHost())
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestScraperScrape(t *testing.T) {
	testCases := []struct {
		desc              string
		setupMockClient   func(t *testing.T) client
		expectedMetricGen func(t *testing.T) pmetric.Metrics
		expectedErr       error
	}{
		{
			desc: "Nil client",
			setupMockClient: func(*testing.T) client {
				return nil
			},
			expectedMetricGen: func(*testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			expectedErr: errClientNotInit,
		},
		{
			desc: "API Call Failure",
			setupMockClient: func(*testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetQueues", mock.Anything).Return(nil, errors.New("some api error"))
				mockClient.On("GetNodes", mock.Anything).Return(nil, errors.New("some api error"))
				return &mockClient
			},
			expectedMetricGen: func(*testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			expectedErr: scrapererror.NewPartialScrapeError(
				errors.New("failed to collect queue metrics: some api error; failed to collect node metrics: some api error"),
				0, // No metrics were collected
			),
		},
		{
			desc: "Successful Queue Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				// use helper function from client tests
				data := loadAPIResponseData(t, queuesAPIResponseFile)
				var queues []*models.Queue
				err := json.Unmarshal(data, &queues)
				require.NoError(t, err)

				mockClient.On("GetQueues", mock.Anything).Return(queues, nil)
				mockClient.On("GetNodes", mock.Anything).Return(nil, nil)
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_golden.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: nil,
		},
		{
			desc: "Successful Queue + Node Metrics Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}

				// Fixed: relative path only
				queueData := loadAPIResponseData(t, queuesAPIResponseFile)
				var queues []*models.Queue
				err := json.Unmarshal(queueData, &queues)
				require.NoError(t, err)

				nodeData := loadAPIResponseData(t, nodesAPIResponseFile)

				var nodes []*models.Node
				err = json.Unmarshal(nodeData, &nodes)
				require.NoError(t, err)

				require.NotEmpty(t, nodes, "Mock node list should not be empty")

				mockClient.On("GetQueues", mock.Anything).Return(queues, nil)
				mockClient.On("GetNodes", mock.Anything).Return(nodes, nil)

				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_golden_queues_nodes.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)

			// Enable all 74 node metrics
			cfg.Metrics.RabbitmqNodeDiskFree.Enabled = true
			cfg.Metrics.RabbitmqNodeDiskFreeLimit.Enabled = true
			cfg.Metrics.RabbitmqNodeDiskFreeAlarm.Enabled = true
			cfg.Metrics.RabbitmqNodeDiskFreeDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeMemUsed.Enabled = true
			cfg.Metrics.RabbitmqNodeMemUsedDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeMemLimit.Enabled = true
			cfg.Metrics.RabbitmqNodeMemAlarm.Enabled = true

			cfg.Metrics.RabbitmqNodeFdUsed.Enabled = true
			cfg.Metrics.RabbitmqNodeFdTotal.Enabled = true
			cfg.Metrics.RabbitmqNodeFdUsedDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeSocketsUsed.Enabled = true
			cfg.Metrics.RabbitmqNodeSocketsTotal.Enabled = true
			cfg.Metrics.RabbitmqNodeSocketsUsedDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeProcUsed.Enabled = true
			cfg.Metrics.RabbitmqNodeProcTotal.Enabled = true
			cfg.Metrics.RabbitmqNodeProcUsedDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeUptime.Enabled = true
			cfg.Metrics.RabbitmqNodeRunQueue.Enabled = true
			cfg.Metrics.RabbitmqNodeProcessors.Enabled = true
			cfg.Metrics.RabbitmqNodeContextSwitches.Enabled = true
			cfg.Metrics.RabbitmqNodeContextSwitchesDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeGcNum.Enabled = true
			cfg.Metrics.RabbitmqNodeGcNumDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeGcBytesReclaimed.Enabled = true
			cfg.Metrics.RabbitmqNodeGcBytesReclaimedDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeIoReadCount.Enabled = true
			cfg.Metrics.RabbitmqNodeIoReadCountDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeIoReadBytes.Enabled = true
			cfg.Metrics.RabbitmqNodeIoReadBytesDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeIoReadAvgTime.Enabled = true
			cfg.Metrics.RabbitmqNodeIoReadAvgTimeDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeIoWriteCount.Enabled = true
			cfg.Metrics.RabbitmqNodeIoWriteCountDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeIoWriteBytes.Enabled = true
			cfg.Metrics.RabbitmqNodeIoWriteBytesDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeIoWriteAvgTime.Enabled = true
			cfg.Metrics.RabbitmqNodeIoWriteAvgTimeDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeIoSyncCount.Enabled = true
			cfg.Metrics.RabbitmqNodeIoSyncCountDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeIoSyncAvgTime.Enabled = true
			cfg.Metrics.RabbitmqNodeIoSyncAvgTimeDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeIoSeekCount.Enabled = true
			cfg.Metrics.RabbitmqNodeIoSeekCountDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeIoSeekAvgTime.Enabled = true
			cfg.Metrics.RabbitmqNodeIoSeekAvgTimeDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeIoReopenCount.Enabled = true
			cfg.Metrics.RabbitmqNodeIoReopenCountDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeMnesiaRAMTxCount.Enabled = true
			cfg.Metrics.RabbitmqNodeMnesiaRAMTxCountDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeMnesiaDiskTxCount.Enabled = true
			cfg.Metrics.RabbitmqNodeMnesiaDiskTxCountDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeMsgStoreReadCount.Enabled = true
			cfg.Metrics.RabbitmqNodeMsgStoreReadCountDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeMsgStoreWriteCount.Enabled = true
			cfg.Metrics.RabbitmqNodeMsgStoreWriteCountDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeQueueIndexWriteCount.Enabled = true
			cfg.Metrics.RabbitmqNodeQueueIndexWriteCountDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeQueueIndexReadCount.Enabled = true
			cfg.Metrics.RabbitmqNodeQueueIndexReadCountDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeConnectionCreated.Enabled = true
			cfg.Metrics.RabbitmqNodeConnectionCreatedDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeConnectionClosed.Enabled = true
			cfg.Metrics.RabbitmqNodeConnectionClosedDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeChannelCreated.Enabled = true
			cfg.Metrics.RabbitmqNodeChannelCreatedDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeChannelClosed.Enabled = true
			cfg.Metrics.RabbitmqNodeChannelClosedDetailsRate.Enabled = true

			cfg.Metrics.RabbitmqNodeQueueDeclared.Enabled = true
			cfg.Metrics.RabbitmqNodeQueueDeclaredDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeQueueCreated.Enabled = true
			cfg.Metrics.RabbitmqNodeQueueCreatedDetailsRate.Enabled = true
			cfg.Metrics.RabbitmqNodeQueueDeleted.Enabled = true
			cfg.Metrics.RabbitmqNodeQueueDeletedDetailsRate.Enabled = true

			scraper := newScraper(zap.NewNop(), cfg, receivertest.NewNopSettings(metadata.Type))
			scraper.client = tc.setupMockClient(t)

			actualMetrics, err := scraper.scrape(context.Background())
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr.Error())
			}

			expectedMetrics := tc.expectedMetricGen(t)

			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
				pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
			))
		})
	}
}
