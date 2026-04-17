package gnmireceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.Equal(t, "gnmi", f.Type().String())
}

func TestCreateMetricsReceiver(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Targets = []TargetConfig{{Address: "localhost:57400", Subscriptions: []SubscriptionConfig{{Path: "/test"}}}}

	set := receivertest.NewNopSettings(component.MustNewType(typeStr))

	mReceiver, err := f.CreateMetrics(
		context.Background(),
		set,
		cfg,
		consumertest.NewNop(),
	)

	assert.NoError(t, err)
	assert.NotNil(t, mReceiver)
}
