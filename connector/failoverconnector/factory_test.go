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
	cfg := &Config{
		PipelinePriority: [][]component.ID{{component.NewIDWithName(component.DataTypeTraces, "0"), component.NewIDWithName(component.DataTypeTraces, "1")}, {component.NewIDWithName(component.DataTypeTraces, "2")}},
		RetryInterval:    5 * time.Minute,
		RetryGap:         10 * time.Second,
		MaxRetries:       5,
	}

	router := connectortest.NewTracesRouter(
		connectortest.WithNopTraces(component.NewIDWithName(component.DataTypeTraces, "0")),
		connectortest.WithNopTraces(component.NewIDWithName(component.DataTypeTraces, "1")),
	)

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Traces))

	assert.NoError(t, err)
	assert.NotNil(t, conn)
}
