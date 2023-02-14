// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver/internal/models"
)

func TestStart(t *testing.T) {
	recv := testRecv()

	err := recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPoll(t *testing.T) {
	testCases := []struct {
		desc       string
		config     *Config
		client     func() client
		goldenFile string
	}{
		{
			desc:   "single log entry",
			config: testConfig(),
			client: func() client {
				tC, err := testClient(singleLog)
				require.Nil(t, err)
				return tC
			},
			goldenFile: "single-processed-logs.json",
		},
		{
			desc:   "multi log entry",
			config: testConfig(),
			client: func() client {
				tC, err := testClient(multiLogs)
				require.Nil(t, err)
				return tC
			},
			goldenFile: "multi-processed-logs.json",
		},
		{
			desc:   "partial log entry",
			config: testConfig(),
			client: func() client {
				tC, err := testClient(partialLogs)
				require.Nil(t, err)
				return tC
			},
			goldenFile: "partial-processed-logs.json",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			logSink := &consumertest.LogsSink{}
			recv := logsReceiver{
				pollInterval:  tc.config.PollInterval,
				nextStartTime: time.Now().Add(-tc.config.PollInterval).Format(time.RFC3339),
				consumer:      logSink,
				wg:            &sync.WaitGroup{},
				doneChan:      make(chan bool),
				id:            receivertest.NewNopCreateSettings().ID,
				storageID:     tc.config.StorageID,
				logger:        zap.NewNop(),
				cfg:           tc.config,
			}
			recv.client = tc.client()

			err := recv.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				return logSink.LogRecordCount() > 0
			}, 10*time.Second, 10*time.Millisecond)

			require.NoError(t, recv.Shutdown(context.Background()))
			logs := logSink.AllLogs()[0]

			expected, err := readLogs(filepath.Join("testdata", "processed", tc.goldenFile))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expected, logs, plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func TestPollError(t *testing.T) {
	mockClient := mocks.MockClient{}
	mockClient.On("MakeRequest", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("an error"))
	recv := testRecv()
	recv.client = &mockClient

	err := recv.poll(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, errors.New("an error"), err.Error())
}

func TestStorageUpdate(t *testing.T) {
	mockClient := mocks.MockClient{}
	mockClient.On("MakeRequest", mock.Anything, mock.Anything, mock.Anything).Return([]*models.Log{}, nil)

	recv := testRecv()
	recv.client = &mockClient
	recv.storageClient = storage.NewNopClient()
	// Expect the new to be replace by the last record time since it is not nil
	recv.record = &logRecord{LastRecordedTime: time.Now()}

	err := recv.poll(context.Background())
	require.Nil(t, err)
	// Expect the last record time to now be nil after used
	require.Nil(t, recv.record)
}

func readLogs(path string) (plog.Logs, error) {
	f, err := os.Open(path)
	if err != nil {
		return plog.Logs{}, err
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return plog.Logs{}, err
	}

	unmarshaler := plog.JSONUnmarshaler{}
	return unmarshaler.UnmarshalLogs(b)
}

func testRecv() logsReceiver {
	cfg := testConfig()
	logSink := &consumertest.LogsSink{}
	return logsReceiver{
		pollInterval:  cfg.PollInterval,
		nextStartTime: time.Now().Add(-cfg.PollInterval).Format(time.RFC3339),
		consumer:      logSink,
		wg:            &sync.WaitGroup{},
		doneChan:      make(chan bool),
		logger:        zap.NewNop(),
		id:            receivertest.NewNopCreateSettings().ID,
		storageID:     cfg.StorageID,
	}
}

func testClient(filePath string) (*mocks.MockClient, error) {
	mockClient := mocks.MockClient{}
	response, err := loadTestFile(filePath)
	if err != nil {
		return nil, err
	}

	mockClient.On("MakeRequest", mock.Anything, mock.Anything, mock.Anything).Return(response, nil)

	return &mockClient, nil
}

func testConfig() *Config {
	return &Config{
		PollInterval: 1 * time.Microsecond,
		Zone:         "023e105f4ecef8ad9ca31a8372d0c353",
		Auth: &Auth{
			XAuthKey:   "abc123",
			XAuthEmail: "email@email.com",
		},
		Logs: &LogsConfig{
			Sample: float32(defaultSampleRate),
			Count:  defaultCount,
			Fields: defaultFields,
		},
	}
}

func TestAddNilValuesToMap(t *testing.T) {
	pLogs := plog.NewLogs()
	resourceLogs := pLogs.ResourceLogs().AppendEmpty()
	logRecord := resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	attrs := logRecord.Attributes()

	t.Run("add nil values to map", func(t *testing.T) {
		nilModel := []*models.Log{
			{},
		}

		putStringToMapNotNil(attrs, "nil", nilModel[0].ClientIP)
		putIntToMapNotNil(attrs, "nil", nilModel[0].EdgeEndTimestamp)

		require.EqualValues(t, 0, attrs.Len())
	})

	t.Run("add non-nil values to map", func(t *testing.T) {
		notNilModel := []*models.Log{
			{
				ClientIP:         &[]string{"89.163.253.200"}[0],
				EdgeEndTimestamp: &[]int64{0}[0],
			},
		}

		putStringToMapNotNil(attrs, "ClientIP", notNilModel[0].ClientIP)
		putIntToMapNotNil(attrs, "EdgeEndTimestamp", notNilModel[0].EdgeEndTimestamp)

		require.EqualValues(t, 2, attrs.Len())
	})
}
