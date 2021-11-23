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

package prometheusexecreceiver

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
)

var (
	wantReceiver2 = &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "test")),
		ScrapeInterval:   60 * time.Second,
		Port:             9104,
		SubprocessConfig: subprocessmanager.SubprocessConfig{
			Command: "mysqld_exporter",
			Env:     []subprocessmanager.EnvConfig{},
		},
	}

	wantReceiver3 = &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "test2")),
		ScrapeInterval:   90 * time.Second,
		SubprocessConfig: subprocessmanager.SubprocessConfig{
			Command: "postgres_exporter",
			Env:     []subprocessmanager.EnvConfig{},
		},
	}

	wantReceiver4 = &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "end_to_end_test/1")),
		ScrapeInterval:   1 * time.Second,
		Port:             9999,
		SubprocessConfig: subprocessmanager.SubprocessConfig{
			Command: "go run ./testdata/end_to_end_metrics_test/test_prometheus_exporter.go {{port}}",
			Env: []subprocessmanager.EnvConfig{
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

	wantReceiver5 = &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "end_to_end_test/2")),
		ScrapeInterval:   1 * time.Second,
		SubprocessConfig: subprocessmanager.SubprocessConfig{
			Command: "go run ./testdata/end_to_end_metrics_test/test_prometheus_exporter.go {{port}}",
			Env:     []subprocessmanager.EnvConfig{},
		},
	}
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory

	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 5)

	receiver1 := cfg.Receivers[config.NewComponentID(typeStr)]
	assert.Equal(t, factory.CreateDefaultConfig(), receiver1)

	receiver2 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "test")]
	assert.Equal(t, wantReceiver2, receiver2)

	receiver3 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "test2")]
	assert.Equal(t, wantReceiver3, receiver3)

	receiver4 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "end_to_end_test/1")]
	assert.Equal(t, wantReceiver4, receiver4)

	receiver5 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "end_to_end_test/2")]
	assert.Equal(t, wantReceiver5, receiver5)
}
