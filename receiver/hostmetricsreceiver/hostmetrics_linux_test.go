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

//go:build linux

package hostmetricsreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"
)

func TestConsistentRootPaths(t *testing.T) {
	env := &testEnv{env: map[string]string{"HOST_PROC": "testdata"}}
	// use testdata because it's a directory that exists - don't actually use any files in it
	assert.Nil(t, testValidate("testdata", env))
	assert.Nil(t, testValidate("", env))
	assert.Nil(t, testValidate("/", env))
}

func TestInconsistentRootPaths(t *testing.T) {
	globalRootPath = "foo"
	err := testValidate("testdata", &testEnv{})
	assert.EqualError(t, err, "inconsistent root_path configuration detected between hostmetricsreceivers: `foo` != `testdata`")
}

func TestLoadConfigRootPath(t *testing.T) {
	t.Setenv("HOST_PROC", "testdata")
	factories, _ := otelcoltest.NopFactories()
	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-root-path.yaml"), factories)
	require.NoError(t, err)
	globalRootPath = ""

	r := cfg.Receivers[component.NewID(typeStr)].(*Config)
	expectedConfig := factory.CreateDefaultConfig().(*Config)
	expectedConfig.RootPath = "testdata"
	cpuScraperCfg := (&cpuscraper.Factory{}).CreateDefaultConfig()
	cpuScraperCfg.SetRootPath("testdata")
	expectedConfig.Scrapers = map[string]internal.Config{cpuscraper.TypeStr: cpuScraperCfg}

	assert.Equal(t, expectedConfig, r)
}

func TestLoadInvalidConfig_RootPathNotExist(t *testing.T) {
	factories, _ := otelcoltest.NopFactories()
	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	_, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-bad-root-path.yaml"), factories)
	assert.ErrorContains(t, err, "invalid root_path:")
	globalRootPath = ""
}

func testValidate(rootPath string, env environment) error {
	err := validateRootPath(rootPath, env)
	globalRootPath = ""
	return err
}
