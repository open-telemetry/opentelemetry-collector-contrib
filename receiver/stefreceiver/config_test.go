// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver"
import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestConfig(t *testing.T) {
	tests := []struct {
		name           string
		expectedConfig *Config
	}{
		{
			name: "endpoint",
			expectedConfig: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ServerConfig.NetAddr.Endpoint = "0.0.0.0:3456"
				return cfg
			}(),
		},
		{
			name: "tls",
			expectedConfig: func() *Config {
				cfg := createDefaultConfig().(*Config)
				tls := configtls.NewDefaultServerConfig()
				cfg.ServerConfig.TLSSetting = &tls
				cfg.ServerConfig.TLSSetting.KeyFile = "server.key"
				return cfg
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			conf, err := cm.Sub(fmt.Sprintf("stef/%s", test.name))
			require.NoError(t, err)
			cfg := createDefaultConfig()
			require.NoError(t, conf.Unmarshal(cfg))
			assert.Equal(t, test.expectedConfig, cfg)
		})
	}
}
