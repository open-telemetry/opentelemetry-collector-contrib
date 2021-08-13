// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunkhecreceiver

import (
	"path"
	"testing"

	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func TestDefaultPath(t *testing.T) {
	c := createDefaultConfig()
	err := c.(*Config).initialize()
	assert.NoError(t, err)
	assert.True(t, c.(*Config).pathGlob.Match("/foo"))
	assert.True(t, c.(*Config).pathGlob.Match("/foo/bar"))
	assert.True(t, c.(*Config).pathGlob.Match("/bar"))
}

func TestBadGlob(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Path = "["
	err := c.initialize()
	assert.Error(t, err)
}

func TestFixedPath(t *testing.T) {
	c := createDefaultConfig()
	c.(*Config).Path = "/foo"
	err := c.(*Config).initialize()
	assert.NoError(t, err)
	assert.True(t, c.(*Config).pathGlob.Match("/foo"))
	assert.False(t, c.(*Config).pathGlob.Match("/foo/bar"))
	assert.False(t, c.(*Config).pathGlob.Match("/bar"))
}

func TestPathWithGlob(t *testing.T) {
	c := createDefaultConfig()
	c.(*Config).Path = "/foo/*"
	err := c.(*Config).initialize()
	assert.NoError(t, err)
	assert.False(t, c.(*Config).pathGlob.Match("/foo"))
	assert.True(t, c.(*Config).pathGlob.Match("/foo/bar"))
	assert.False(t, c.(*Config).pathGlob.Match("/bar"))
}

func TestInvalidPathGlobPattern(t *testing.T) {
	c := Config{Path: "**/ "}
	err := c.initialize()
	assert.Error(t, err)
}

func TestInvalidPathSpaces(t *testing.T) {
	c := Config{Path: "  foo  "}
	err := c.initialize()
	assert.Error(t, err)
}

func TestCreateValidEndpoint(t *testing.T) {
	endpoint, err := extractPortFromEndpoint("localhost:123")
	assert.NoError(t, err)
	assert.Equal(t, 123, endpoint)
}

func TestCreateInvalidEndpoint(t *testing.T) {
	endpoint, err := extractPortFromEndpoint("")
	assert.EqualError(t, err, "endpoint is not formatted correctly: missing port in address")
	assert.Equal(t, 0, endpoint)
}

func TestCreateNoPort(t *testing.T) {
	endpoint, err := extractPortFromEndpoint("localhost:")
	assert.EqualError(t, err, "endpoint port is not a number: strconv.ParseInt: parsing \"\": invalid syntax")
	assert.Equal(t, 0, endpoint)
}

func TestCreateLargePort(t *testing.T) {
	endpoint, err := extractPortFromEndpoint("localhost:65536")
	assert.EqualError(t, err, "port number must be between 1 and 65535")
	assert.Equal(t, 0, endpoint)
}

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 3)

	r0 := cfg.Receivers[config.NewID(typeStr)].(*Config)
	assert.Equal(t, r0, createDefaultConfig())
	assert.NoError(t, r0.initialize())

	r1 := cfg.Receivers[config.NewIDWithName(typeStr, "allsettings")].(*Config)
	assert.NoError(t, r1.initialize())
	expectedAllSettings := &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewIDWithName(typeStr, "allsettings")),
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "localhost:8088",
		},
		AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
			AccessTokenPassthrough: true,
		},
		Path:          "/foo",
		SourceKey:     "file.name",
		SourceTypeKey: "foobar",
		IndexKey:      "myindex",
		HostKey:       "myhostfield",
	}
	expectedAllSettings.pathGlob, _ = glob.Compile("/foo")
	assert.Equal(t, expectedAllSettings, r1)

	r2 := cfg.Receivers[config.NewIDWithName(typeStr, "tls")].(*Config)
	assert.NoError(t, r2.initialize())
	expectedTLSConfig := &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewIDWithName(typeStr, "tls")),
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: ":8088",
			TLSSetting: &configtls.TLSServerSetting{
				TLSSetting: configtls.TLSSetting{
					CertFile: "/test.crt",
					KeyFile:  "/test.key",
				},
			},
		},
		AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
			AccessTokenPassthrough: false,
		},
		Path:          "",
		SourceKey:     "com.splunk.source",
		SourceTypeKey: "com.splunk.sourcetype",
		IndexKey:      "com.splunk.index",
		HostKey:       "host.name",
	}
	expectedTLSConfig.pathGlob, _ = glob.Compile("*")
	assert.Equal(t, expectedTLSConfig, r2)
}
