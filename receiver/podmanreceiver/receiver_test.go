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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper"
)

func TestNewReceiver(t *testing.T) {
	config := &Config{
		Endpoint: "unix:///run/some.sock",
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 1 * time.Second,
		},
	}
	nextConsumer := consumertest.NewNop()
	mr, err := newReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), config, nextConsumer, nil)

	assert.NotNil(t, mr)
	assert.Nil(t, err)

	receiver := mr.(*receiver)
	assert.Equal(t, config, receiver.config)
	assert.Same(t, nextConsumer, receiver.nextConsumer)
}

func TestNewReceiverErrors(t *testing.T) {
	r, err := newReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), &Config{}, consumertest.NewNop(), nil)
	assert.Nil(t, r)
	require.Error(t, err)
	assert.Equal(t, "config.Endpoint must be specified", err.Error())

	r, err = newReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), &Config{Endpoint: "someEndpoint"}, consumertest.NewNop(), nil)
	assert.Nil(t, r)
	require.Error(t, err)
	assert.Equal(t, "config.CollectionInterval must be specified", err.Error())
}

func TestScraperLoop(t *testing.T) {
	cfg := createDefaultConfig()
	cfg.CollectionInterval = 100 * time.Millisecond

	client := make(mockClient)
	consumer := make(mockConsumer)

	r, err := newReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, consumer, client.factory)
	assert.NotNil(t, r)
	require.NoError(t, err)

	go func() {
		client <- containerStatsReport{
			Stats: []containerStats{{
				ContainerID: "c1",
			}},
			Error: "",
		}
	}()

	r.Start(context.Background(), componenttest.NewNopHost())

	md := <-consumer
	assert.Equal(t, md.ResourceMetrics().Len(), 1)

	r.Shutdown(context.Background())
}

type mockClient chan containerStatsReport

func (c mockClient) factory(logger *zap.Logger, cfg *Config) (client, error) {
	return c, nil
}

func (c mockClient) stats() ([]containerStats, error) {
	report := <-c
	if report.Error != "" {
		return nil, errors.New(report.Error)
	}
	return report.Stats, nil
}

type mockConsumer chan pdata.Metrics

func (m mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (m mockConsumer) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	m <- md
	return nil
}
