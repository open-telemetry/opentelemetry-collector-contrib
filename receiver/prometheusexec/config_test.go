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

package prometheusexec

import (
	"path"
	"testing"
	"time"

	subconfig "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexec/subprocessmanager/config"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configmodels"
)

var (
	wantReceiver2 = &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: configmodels.Type("prometheus_exec"),
			NameVal: "prometheus_exec/test",
		},
		ScrapeInterval: 60 * time.Second,
		SubprocessConfig: subconfig.SubprocessConfig{
			Command:    "mysqld_exporter",
			Port:       9104,
			CustomName: "",
			Env:        []subconfig.EnvConfig{},
		},
	}

	wantReceiver3 = &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: configmodels.Type("prometheus_exec"),
			NameVal: "prometheus_exec/test2",
		},
		ScrapeInterval: 90 * time.Second,
		SubprocessConfig: subconfig.SubprocessConfig{
			Command:    "postgres_exporter",
			Port:       9187,
			CustomName: "",
			Env:        []subconfig.EnvConfig{},
		},
	}

	wantReceiver4 = &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: configmodels.Type("prometheus_exec"),
			NameVal: "prometheus_exec/test2/secondary",
		},
		ScrapeInterval: 60 * time.Second,
		SubprocessConfig: subconfig.SubprocessConfig{
			Command:    "go run testdata/main.go",
			CustomName: "",
			Env: []subconfig.EnvConfig{
				{
					Name:  "DATA_SOURCE_NAME",
					Value: "user:password@(hostname:port)/dbname",
				},
				{
					Name:  "SECONDARY_PORT",
					Value: "1234",
				},
			},
		},
	}
)

func TestLoadConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.NoError(t, err)

	factory := &Factory{}
	receiverType := "prometheus_exec"
	factories.Receivers[configmodels.Type(receiverType)] = factory

	config, err := config.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	assert.NoError(t, err)
	assert.NotNil(t, config)

	assert.Equal(t, len(config.Receivers), 4)

	receiver1 := config.Receivers[receiverType]
	assert.Equal(t, factory.CreateDefaultConfig(), receiver1)

	receiver2 := config.Receivers["prometheus_exec/test"]
	assert.Equal(t, wantReceiver2, receiver2)

	receiver3 := config.Receivers["prometheus_exec/test2"]
	assert.Equal(t, wantReceiver3, receiver3)

	receiver4 := config.Receivers["prometheus_exec/test2/secondary"]
	assert.Equal(t, wantReceiver4, receiver4)
}
