package netflowreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)


func TestCreateValidDefaultReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := receivertest.NewNopSettings()
	receiver, err := factory.CreateLogsReceiver(context.Background(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, receiver, "receiver creation failed")
	// TODO - Will be added on the following PR
	// assert.NotNil(t, "sflow", receiver.(*netflowReceiver).listeners[0].recv)

}