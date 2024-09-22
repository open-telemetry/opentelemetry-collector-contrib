// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package podmanreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewReceiver(t *testing.T) {
	mr, err := newMetricsReceiver(receivertest.NewNopSettings(), &Config{}, consumertest.NewNop(), nil)
	assert.Nil(t, mr)
	require.Error(t, err)
	assert.Equal(t, "podman receiver is not supported on windows", err.Error())
}
