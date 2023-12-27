// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
)

func TestNewFactory(t *testing.T) {
	traces0 := component.NewIDWithName(component.DataTypeTraces, "0")
	traces1 := component.NewIDWithName(component.DataTypeTraces, "1")
	traces2 := component.NewIDWithName(component.DataTypeTraces, "2")
	cfg := &Config{
		PipelinePriority: [][]component.ID{{traces0, traces1}, {traces2}},
		RetryInterval:    5 * time.Minute,
		RetryGap:         10 * time.Second,
		MaxRetries:       5,
	}

	router := connectortest.NewTracesRouter(
		connectortest.WithNopTraces(traces0),
		connectortest.WithNopTraces(traces1),
		connectortest.WithNopTraces(traces2),
	)

	conn, err := NewFactory().CreateTracesToTraces(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Traces))

	assert.NoError(t, err)
	assert.NotNil(t, conn)
}
