// Copyright  The OpenTelemetry Authors
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

//go:build linux

package hostmetricsreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"
)

func TestConsistentRootPaths(t *testing.T) {
	// use testdata because it's a directory that exists - don't actually use any files in it
	t.Setenv("HOST_PROC", "testdata/proc")
	assert.Nil(t, validateRootPath("testdata"))
	assert.Nil(t, validateRootPath(""))
}

func TestInconsistentRootPaths(t *testing.T) {
	err := validateRootPath("testdata")
	assert.EqualError(t, err, "config `root_path=testdata` is inconsistent with envvar `HOST_PROC=` config, root_path must be the prefix of the environment variable")

	t.Setenv("HOST_PROC", "doesnt-start-with-testdata")
	err = validateRootPath("testdata")
	assert.EqualError(t, err, "config `root_path=testdata` is inconsistent with envvar `HOST_PROC=doesnt-start-with-testdata` config, root_path must be the prefix of the environment variable")

	t.Setenv("HOST_PROC", "testdata")
	t.Setenv("HOST_SYS", "doesnt-start-with-testdata")
	err = validateRootPath("testdata")
	assert.EqualError(t, err, "config `root_path=testdata` is inconsistent with envvar `HOST_SYS=doesnt-start-with-testdata` config, root_path must be the prefix of the environment variable")
}

func TestLoadConfigRootPath(t *testing.T) {
	t.Setenv("HOST_PROC", "testdata")
	factories, _ := componenttest.NopFactories()
	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config-root-path.yaml"), factories)
	require.NoError(t, err)

	r := cfg.Receivers[config.NewComponentID(typeStr)].(*Config)
	expectedConfig := factory.CreateDefaultConfig().(*Config)
	expectedConfig.RootPath = "testdata"
	cpuScraperCfg := (&cpuscraper.Factory{}).CreateDefaultConfig()
	cpuScraperCfg.SetRootPath("testdata")
	expectedConfig.Scrapers = map[string]internal.Config{cpuscraper.TypeStr: cpuScraperCfg}

	assert.Equal(t, expectedConfig, r)
}

func TestLoadInvalidConfig_RootPathNotExist(t *testing.T) {
	factories, _ := componenttest.NopFactories()
	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	_, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config-bad-root-path.yaml"), factories)
	assert.ErrorContains(t, err, "invalid root_path:")
}
