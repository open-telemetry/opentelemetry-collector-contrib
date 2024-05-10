// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com.open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

func Test_NewFactory(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, metadata.Type, f.Type())
}

func Test_NewLogsReceiver(t *testing.T) {
	f := NewFactory()
	config := createDefaultConfig().(*Config)
	config.Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234;EntityPath=hubName"

	receiver, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), config, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, receiver)
}

func Test_NewMetricsReceiver(t *testing.T) {
	f := NewFactory()
	config := createDefaultConfig().(*Config)
	config.Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234;EntityPath=hubName"

	receiver, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), config, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, receiver)
}
