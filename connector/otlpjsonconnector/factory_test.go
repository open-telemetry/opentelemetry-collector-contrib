// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonconnector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestNewFactory(t *testing.T) {
	cfg := &Config{}

	lc, err := consumer.NewLogs(func(context.Context, plog.Logs) error {
		return nil
	})
	assert.NoError(t, err)

	factory := NewFactory()
	conn, err := factory.CreateLogsToLogs(context.Background(),
		connectortest.NewNopSettings(), cfg, lc)

	assert.NoError(t, err)
	assert.NotNil(t, conn)
}
