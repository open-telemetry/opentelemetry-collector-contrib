// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicegraphconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	conn, err := factory.CreateTracesToMetrics(
		context.Background(),
		connectortest.NewNopCreateSettings(),
		&servicegraphprocessor.Config{},
		consumertest.NewNop(),
	)

	assert.NoError(t, err)
	assert.NotNil(t, conn)
}
