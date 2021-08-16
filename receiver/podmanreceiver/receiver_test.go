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

// +build !windows
// TODO review if tests should succeed on Windows

package podmanreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.opentelemetry.io/collector/testbed/testbed"
	"go.uber.org/zap"
)

func TestNewReceiver(t *testing.T) {
	config := &Config{
		Endpoint: "unix:///run/some.sock",
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 1 * time.Second,
		},
	}
	logger := zap.NewNop()
	nextConsumer := &testbed.MockMetricConsumer{}
	mr, err := NewReceiver(context.Background(), logger, config, nextConsumer)
	assert.NotNil(t, mr)
	assert.Nil(t, err)

	receiver := mr.(*Receiver)
	assert.Equal(t, config, receiver.config)
	assert.Equal(t, nextConsumer, receiver.nextConsumer)
	assert.Equal(t, logger, receiver.logger)
}

func TestNewReceiverErrors(t *testing.T) {
	logger := zap.NewNop()

	r, err := NewReceiver(context.Background(), logger, &Config{}, &testbed.MockMetricConsumer{})
	assert.Nil(t, r)
	require.Error(t, err)
	assert.Equal(t, "config.Endpoint must be specified", err.Error())

	r, err = NewReceiver(context.Background(), logger, &Config{Endpoint: "someEndpoint"}, &testbed.MockMetricConsumer{})
	assert.Nil(t, r)
	require.Error(t, err)
	assert.Equal(t, "config.CollectionInterval must be specified", err.Error())
}
