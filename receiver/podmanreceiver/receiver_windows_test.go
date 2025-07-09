// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package podmanreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver/internal/metadata"
)

func TestNewReceiver(t *testing.T) {
	mr, err := newMetricsReceiver(receivertest.NewNopSettings(metadata.Type), &Config{}, consumertest.NewNop(), nil)
	assert.Nil(t, mr)
	assert.Error(t, err)
	assert.Equal(t, "podman receiver is not supported on windows", err.Error())
}
