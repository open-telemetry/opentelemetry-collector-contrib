// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resourcedetectionprocessor

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory

	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	p1 := cfg.Processors["resourcedetection"]
	assert.Equal(t, p1, factory.CreateDefaultConfig())

	p2 := cfg.Processors["resourcedetection/gce"]
	assert.Equal(t, p2, &Config{
		ProcessorSettings: configmodels.ProcessorSettings{
			TypeVal: "resourcedetection",
			NameVal: "resourcedetection/gce",
		},
		Detectors: []string{"env", "gce"},
		Timeout:   2 * time.Second,
		Override:  false,
	})

	p3 := cfg.Processors["resourcedetection/ec2"]
	assert.Equal(t, p3, &Config{
		ProcessorSettings: configmodels.ProcessorSettings{
			TypeVal: "resourcedetection",
			NameVal: "resourcedetection/ec2",
		},
		Detectors: []string{"env", "ec2"},
		Timeout:   2 * time.Second,
		Override:  false,
	})
}
