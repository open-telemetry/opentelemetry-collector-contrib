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

package jmxmetricsreceiver

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
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 5)

	r0 := cfg.Receivers["jmx_metrics"].(*config)
	require.NoError(t, configcheck.ValidateConfig(r0))
	assert.Equal(t, r0, factory.CreateDefaultConfig())
	err = r0.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx_metrics missing required fields: [`service_url` `groovy_script`]", err.Error())

	r1 := cfg.Receivers["jmx_metrics/all"].(*config)
	require.NoError(t, configcheck.ValidateConfig(r1))
	require.NoError(t, r1.validate())
	assert.Equal(t, r1,
		&config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: "jmx_metrics",
				NameVal: "jmx_metrics/all",
			},
			ServiceURL:         "myserviceurl",
			GroovyScript:       "mygroovyscriptpath",
			CollectorEndpoint:  "mycollectorendpoint:55680",
			Interval:           15 * time.Second,
			KeystorePath:       "mykeystorepath",
			KeystorePassword:   "mykeystorepassword",
			KeystoreType:       "mykeystoretype",
			TruststorePath:     "mytruststorepath",
			TruststorePassword: "mytruststorepassword",
			RemoteProfile:      "myremoteprofile",
			Realm:              "myrealm",
		})

	r2 := cfg.Receivers["jmx_metrics/missingservice"].(*config)
	require.NoError(t, configcheck.ValidateConfig(r2))
	assert.Equal(t, r2,
		&config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: "jmx_metrics",
				NameVal: "jmx_metrics/missingservice",
			},
			GroovyScript: "mygroovyscriptpath",
			Interval:     10 * time.Second,
		})
	err = r2.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx_metrics/missingservice missing required field: [`service_url`]", err.Error())

	r3 := cfg.Receivers["jmx_metrics/missinggroovy"].(*config)
	require.NoError(t, configcheck.ValidateConfig(r3))
	assert.Equal(t, r3,
		&config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: "jmx_metrics",
				NameVal: "jmx_metrics/missinggroovy",
			},
			ServiceURL: "myserviceurl",
			Interval:   10 * time.Second,
		})
	err = r3.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx_metrics/missinggroovy missing required field: [`groovy_script`]", err.Error())

	r4 := cfg.Receivers["jmx_metrics/invalidinterval"].(*config)
	require.NoError(t, configcheck.ValidateConfig(r4))
	assert.Equal(t, r4,
		&config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: "jmx_metrics",
				NameVal: "jmx_metrics/invalidinterval",
			},
			ServiceURL:   "myserviceurl",
			GroovyScript: "mygroovyscriptpath",
			Interval:     -100 * time.Millisecond,
		})
	err = r4.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx_metrics/invalidinterval `interval` must be positive: -100ms", err.Error())
}
