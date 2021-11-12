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

package jmxreceiver

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 6)

	r0 := cfg.Receivers[config.NewComponentID(typeStr)].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r0))
	assert.Equal(t, r0, factory.CreateDefaultConfig())
	err = r0.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx missing required fields: `endpoint`, `target_system` or `groovy_script`", err.Error())

	r1 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "all")].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r1))
	require.NoError(t, r1.validate())
	assert.Equal(t,
		&Config{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "all")),
			JARPath:            "myjarpath",
			Endpoint:           "myendpoint:12345",
			GroovyScript:       "mygroovyscriptpath",
			CollectionInterval: 15 * time.Second,
			Username:           "myusername",
			Password:           "mypassword",
			OTLPExporterConfig: otlpExporterConfig{
				Endpoint: "myotlpendpoint",
				Headers: map[string]string{
					"x-header-1": "value1",
					"x-header-2": "value2",
				},
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 5 * time.Second,
				},
			},
			KeystorePath:       "mykeystorepath",
			KeystorePassword:   "mykeystorepassword",
			KeystoreType:       "mykeystoretype",
			TruststorePath:     "mytruststorepath",
			TruststorePassword: "mytruststorepassword",
			RemoteProfile:      "myremoteprofile",
			Realm:              "myrealm",
			Properties: map[string]string{
				"property.one":                           "value.one",
				"property.two":                           "value.two.a=value.two.b,value.two.c=value.two.d",
				"org.slf4j.simpleLogger.defaultLogLevel": "info",
			},
		}, r1)

	assert.Equal(
		t, []string{"-Dorg.slf4j.simpleLogger.defaultLogLevel=info", "-Dproperty.one=value.one", "-Dproperty.two=value.two.a=value.two.b,value.two.c=value.two.d"},
		r1.parseProperties(),
	)

	r2 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "missingendpoint")].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r2))
	assert.Equal(t,
		&Config{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "missingendpoint")),
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			GroovyScript:       "mygroovyscriptpath",
			CollectionInterval: 10 * time.Second,
			Properties:         map[string]string{"org.slf4j.simpleLogger.defaultLogLevel": "info"},
			OTLPExporterConfig: otlpExporterConfig{
				Endpoint: "0.0.0.0:0",
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 5 * time.Second,
				},
			},
		}, r2)
	err = r2.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx/missingendpoint missing required field: `endpoint`", err.Error())

	r3 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "missinggroovy")].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r3))
	assert.Equal(t,
		&Config{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "missinggroovy")),
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			Endpoint:           "service:jmx:rmi:///jndi/rmi://host:12345/jmxrmi",
			Properties:         map[string]string{"org.slf4j.simpleLogger.defaultLogLevel": "info"},
			CollectionInterval: 10 * time.Second,
			OTLPExporterConfig: otlpExporterConfig{
				Endpoint: "0.0.0.0:0",
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 5 * time.Second,
				},
			},
		}, r3)
	err = r3.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx/missinggroovy missing required field: `target_system` or `groovy_script`", err.Error())

	r4 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "invalidinterval")].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r4))
	assert.Equal(t,
		&Config{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "invalidinterval")),
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			Endpoint:           "myendpoint:23456",
			GroovyScript:       "mygroovyscriptpath",
			Properties:         map[string]string{"org.slf4j.simpleLogger.defaultLogLevel": "info"},
			CollectionInterval: -100 * time.Millisecond,
			OTLPExporterConfig: otlpExporterConfig{
				Endpoint: "0.0.0.0:0",
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 5 * time.Second,
				},
			},
		}, r4)
	err = r4.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx/invalidinterval `interval` must be positive: -100ms", err.Error())

	r5 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "invalidotlptimeout")].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r5))
	assert.Equal(t,
		&Config{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "invalidotlptimeout")),
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			Endpoint:           "myendpoint:34567",
			GroovyScript:       "mygroovyscriptpath",
			Properties:         map[string]string{"org.slf4j.simpleLogger.defaultLogLevel": "info"},
			CollectionInterval: 10 * time.Second,
			OTLPExporterConfig: otlpExporterConfig{
				Endpoint: "0.0.0.0:0",
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: -100 * time.Millisecond,
				},
			},
		}, r5)
	err = r5.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx/invalidotlptimeout `otlp.timeout` must be positive: -100ms", err.Error())
}
