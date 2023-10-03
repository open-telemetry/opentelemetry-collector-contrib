package failoverconnector

import (
	"context"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"testing"
	"time"
)

func TestNewFactory(t *testing.T) {
	cfg := &Config{
		PipelinePriority: [][]component.ID{{component.NewIDWithName(component.DataTypeTraces, "0"), component.NewIDWithName(component.DataTypeTraces, "1")}, {component.NewIDWithName(component.DataTypeTraces, "2")}},
		RetryInterval:    5 * time.Minute,
		RetryGap:         10 * time.Second,
		MaxRetry:         5,
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
