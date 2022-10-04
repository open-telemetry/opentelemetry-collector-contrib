package k8sobjectreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig()
	rCfg, ok := cfg.(*Config)
	require.True(t, ok)

	assert.Equal(t, &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID("k8sobjects")),
	}, rCfg)
}

func TestFactoryType(t *testing.T) {
	t.Parallel()
	assert.Equal(t, config.Type("k8sobjects"), NewFactory().Type())
}

func TestCreateReceiver(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)

	// Fails with bad K8s Config.
	r, err := createLogsReceiver(
		context.Background(), componenttest.NewNopReceiverCreateSettings(),
		rCfg, consumertest.NewNop(),
	)
	assert.Error(t, err)
	assert.Nil(t, r)

	// Override for test.
	rCfg.makeDynamicClient = newMockDynamicClient().getMockDynamicClient

	r, err = createLogsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		rCfg, consumertest.NewNop(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, r)
}
