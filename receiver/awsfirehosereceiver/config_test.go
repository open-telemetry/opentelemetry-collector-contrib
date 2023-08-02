// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.NoError(t, component.ValidateConfig(cfg))

	require.Equal(t, &Config{
		RecordType: "cwmetrics",
		AccessKey:  "some_access_key",
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "0.0.0.0:4433",
			TLSSetting: &configtls.TLSServerSetting{
				TLSSetting: configtls.TLSSetting{
					CertFile: "server.crt",
					KeyFile:  "server.key",
				},
			},
		},
	}, cfg)
}
