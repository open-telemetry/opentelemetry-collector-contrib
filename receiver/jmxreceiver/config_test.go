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
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 6)

	r0 := cfg.Receivers["jmx"].(*config)
	require.NoError(t, configcheck.ValidateConfig(r0))
	assert.Equal(t, r0, factory.CreateDefaultConfig())
	err = r0.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx missing required fields: `endpoint`, `target_system` or `groovy_script`", err.Error())

	r1 := cfg.Receivers["jmx/all"].(*config)
	require.NoError(t, configcheck.ValidateConfig(r1))
	require.NoError(t, r1.validate())
	assert.Equal(t,
		&config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: "jmx",
				NameVal: "jmx/all",
			},
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
		}, r1)

	r2 := cfg.Receivers["jmx/missingendpoint"].(*config)
	require.NoError(t, configcheck.ValidateConfig(r2))
	assert.Equal(t,
		&config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: "jmx",
				NameVal: "jmx/missingendpoint",
			},
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			GroovyScript:       "mygroovyscriptpath",
			CollectionInterval: 10 * time.Second,
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

	r3 := cfg.Receivers["jmx/missinggroovy"].(*config)
	require.NoError(t, configcheck.ValidateConfig(r3))
	assert.Equal(t,
		&config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: "jmx",
				NameVal: "jmx/missinggroovy",
			},
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			Endpoint:           "service:jmx:rmi:///jndi/rmi://host:12345/jmxrmi",
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

	r4 := cfg.Receivers["jmx/invalidinterval"].(*config)
	require.NoError(t, configcheck.ValidateConfig(r4))
	assert.Equal(t,
		&config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: "jmx",
				NameVal: "jmx/invalidinterval",
			},
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			Endpoint:           "myendpoint:23456",
			GroovyScript:       "mygroovyscriptpath",
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

	r5 := cfg.Receivers["jmx/invalidotlptimeout"].(*config)
	require.NoError(t, configcheck.ValidateConfig(r5))
	assert.Equal(t,
		&config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: "jmx",
				NameVal: "jmx/invalidotlptimeout",
			},
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			Endpoint:           "myendpoint:34567",
			GroovyScript:       "mygroovyscriptpath",
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
