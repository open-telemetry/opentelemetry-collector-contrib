// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver 

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

// only one validate check so far
func testValidateConfig(t *testing.T) {
    t.Parallel()

    // in case we need to add more tests this can just be extended with more cases
	tests := []struct {
		desc   string
		expect error
		conf   Config
	}{
		{
            desc:   "Missing valid endpoint",
			expect: errMissingEndpointFromConfig,
			conf: Config{
                HTTPServerSettings: confighttp.HTTPServerSettings{
                    Endpoint: "",
                },
			},
		},   
    }

    for _, test := range tests {
        t.Run(test.desc, func(t *testing.T) {
            err := test.conf.Validate()
            require.Error(t, err)
            require.Contains(t, err.Error(), test.expect.Error())
        })
    }
}

func testLoadConfig(t *testing.T) {
    t.Parallel()

    cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	// LoadConf includes the TypeStr which NewFactory does not set
	id := component.NewIDWithName(typeStr, "")
	cmNoStr, err := cm.Sub(id.String())
	require.NoError(t, err)

    expect := &Config{
        HTTPServerSettings: confighttp.HTTPServerSettings{
            Endpoint: "0.0.0.0:8080",
        },
        ReadTimeout:  "500",
        WriteTimeout: "500",
        Path:         "some/path",
        HealthPath:   "health/path",
    }

    // create expected config
    factory := NewFactory()
    conf := factory.CreateDefaultConfig()
    require.NoError(t, component.UnmarshalConfig(cmNoStr, conf))
    require.NoError(t, component.ValidateConfig(conf))

    require.Equal(t, expect, conf)
}
