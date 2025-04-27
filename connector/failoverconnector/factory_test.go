// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	traces0 := pipeline.NewIDWithName(pipeline.SignalTraces, "0")
	traces1 := pipeline.NewIDWithName(pipeline.SignalTraces, "1")
	traces2 := pipeline.NewIDWithName(pipeline.SignalTraces, "2")
	cfg := &Config{
		PipelinePriority: [][]pipeline.ID{{traces0, traces1}, {traces2}},
		RetryInterval:    5 * time.Minute,
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		traces0: consumertest.NewNop(),
		traces1: consumertest.NewNop(),
		traces2: consumertest.NewNop(),
	})

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	assert.NoError(t, err)
	assert.NotNil(t, conn)
}
