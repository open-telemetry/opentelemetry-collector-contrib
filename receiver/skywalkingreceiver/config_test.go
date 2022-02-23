package skywalkingreceiver

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/service/servicetest"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 3)

	r1 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "customname")].(*Config)
	assert.Equal(t, r1,
		&Config{
			ReceiverSettings: config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "customname")),
			Protocols: Protocols{
				HTTP: &confighttp.HTTPServerSettings{
					Endpoint: "0.0.0.0:12801",
				},
			},
		})

	rDefaults := cfg.Receivers[config.NewComponentIDWithName(typeStr, "defaults")].(*Config)
	assert.Equal(t, rDefaults,
		&Config{
			ReceiverSettings: config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "defaults")),
			Protocols: Protocols{
				GRPC: &configgrpc.GRPCServerSettings{
					NetAddr: confignet.NetAddr{
						Endpoint:  defaultGRPCBindEndpoint,
						Transport: "tcp",
					},
				},
				HTTP: &confighttp.HTTPServerSettings{
					Endpoint: defaultHTTPBindEndpoint,
				},
			},
		})
}
