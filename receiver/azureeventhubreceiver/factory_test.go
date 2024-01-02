// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func Test_NewFactory(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, component.Type("azureeventhub"), f.Type())
}

func Test_NewLogsReceiver(t *testing.T) {
	f := NewFactory()
	receiver, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), f.CreateDefaultConfig(), consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, receiver)
}

func Test_NewMetricsReceiver(t *testing.T) {
	f := NewFactory()
	receiver, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), f.CreateDefaultConfig(), consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, receiver)
}
