// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

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
	receiver, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(metadata.Type), f.CreateDefaultConfig(), consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, receiver)
}

func Test_NewMetricsReceiver(t *testing.T) {
	f := NewFactory()
	receiver, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), f.CreateDefaultConfig(), consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, receiver)
}
