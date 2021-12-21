// Copyright 2020 OpenTelemetry Authors
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

//go:build !windows
// +build !windows

package podmanreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
)

func TestNewReceiver(t *testing.T) {
	config := &Config{
		Endpoint: "unix:///run/some.sock",
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 1 * time.Second,
		},
	}
	nextConsumer := consumertest.NewNop()
	mr, err := newReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), config, nil)
	mr.registerMetricsConsumer(nextConsumer, componenttest.NewNopReceiverCreateSettings())

	assert.NotNil(t, mr)
	assert.Nil(t, err)
}

func TestNewReceiverErrors(t *testing.T) {
	r, err := newReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), &Config{}, nil)
	assert.Nil(t, r)
	require.Error(t, err)
	assert.Equal(t, "config.Endpoint must be specified", err.Error())

	r, err = newReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), &Config{Endpoint: "someEndpoint"}, nil)
	assert.Nil(t, r)
	require.Error(t, err)
	assert.Equal(t, "config.CollectionInterval must be specified", err.Error())
}

func TestScraperLoop(t *testing.T) {
	cfg := createDefaultConfig()
	cfg.CollectionInterval = 100 * time.Millisecond

	client := make(mockClient)
	consumerForMetrics := make(mockConsumer)

	r, err := newReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, client.factory)
	require.NoError(t, err)
	err = r.registerMetricsConsumer(consumerForMetrics, componenttest.NewNopReceiverCreateSettings())
	require.NoError(t, err)
	assert.NotNil(t, r)

	go func() {
		client <- containerStatsReport{
			Stats: []containerStats{{
				ContainerID: "c1",
			}},
			Error: "",
		}
	}()
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	md := <-consumerForMetrics
	assert.Equal(t, md.ResourceMetrics().Len(), 1)

	err = r.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestLogsLoop(t *testing.T) {
	cfg := createDefaultConfig()

	client := make(mockClientLogs)
	consumerForLogs := make(mockConsumerLogs)

	r, err := newReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, client.factory)
	require.NoError(t, err)
	r.registerLogsConsumer(consumerForLogs)
	assert.NotNil(t, r)

	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	md := <-consumerForLogs
	assert.Equal(t, md.ResourceLogs().Len(), 1)

	md = <-consumerForLogs
	assert.Equal(t, md.ResourceLogs().Len(), 1)

	err = r.Shutdown(context.Background())
	require.NoError(t, err)
}

type mockClient chan containerStatsReport
type mockClientLogs chan http.Response

func (c mockClient) factory(logger *zap.Logger, cfg *Config) (client, error) {
	return c, nil
}

func (c mockClientLogs) factory(logger *zap.Logger, cfg *Config) (client, error) {
	return c, nil
}

func (c mockClient) stats() ([]containerStats, error) {
	report := <-c
	if report.Error != "" {
		return nil, errors.New(report.Error)
	}
	return report.Stats, nil
}

func (c mockClientLogs) stats() ([]containerStats, error) {
	return nil, nil
}

func (c mockClient) getEventsResponse(ctx context.Context) (*http.Response, error) {
	return nil, nil
}

func (c mockClientLogs) getEventsResponse(ctx context.Context) (*http.Response, error) {
	tempReport := &http.Response{
		Status:     "Ok",
		StatusCode: 200,
		Body:       testEventReader{},
	}
	return tempReport, nil
}

type mockConsumer chan pdata.Metrics
type mockConsumerLogs chan pdata.Logs

func (m mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (m mockConsumerLogs) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (m mockConsumer) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	m <- md
	return nil
}

func (m mockConsumerLogs) ConsumeLogs(ctx context.Context, ld pdata.Logs) error {
	m <- ld
	return nil
}

type testEventReader struct{}

var count = 0

func (t testEventReader) Read(b []byte) (int, error) {
	mockRes := event{
		Type: "container",
	}

	encodedResponse, _ := json.Marshal(mockRes)
	if count == 2 {
		return 0, io.EOF
	}

	count++
	n := copy(b, encodedResponse)
	return n, nil
}

func (t testEventReader) Close() error {
	return nil
}
