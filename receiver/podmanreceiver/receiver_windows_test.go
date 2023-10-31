// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package podmanreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewReceiver(t *testing.T) {
	mr, err := newMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), &Config{}, consumertest.NewNop(), nil)
	assert.Nil(t, mr)
	assert.Error(t, err)
	assert.Equal(t, "podman receiver is not supported on windows", err.Error())
}
