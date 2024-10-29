// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudlogsreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
	assert.Equal(t, component.MustNewType("huaweicloudlogsreceiver"), factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig()
	assert.NotNil(t, config)
	assert.NoError(t, componenttest.CheckConfigStruct(config))
}

func TestCreateLogsReceiver(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig()

	rConfig := config.(*Config)
	rConfig.CollectionInterval = 60 * time.Second
	rConfig.InitialDelay = time.Second

	nextConsumer := new(consumertest.LogsSink)
	receiver, err := factory.CreateLogsReceiver(context.Background(), receivertest.NewNopSettings(), config, nextConsumer)
	assert.NoError(t, err)
	assert.NotNil(t, receiver)
}
