// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hostmetricsreceiver

import (
	"path/filepath"
	"testing"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"
)

func TestConsistentRootPaths(t *testing.T) {
	// use testdata because it's a directory that exists - don't actually use any files in it
	assert.NoError(t, testValidate("testdata"))
	assert.NoError(t, testValidate(""))
	assert.NoError(t, testValidate("/"))
}

func TestInconsistentRootPaths(t *testing.T) {
	globalRootPath = "foo"
	err := testValidate("testdata")
	assert.EqualError(t, err, "inconsistent root_path configuration detected between hostmetricsreceivers: `foo` != `testdata`")
}

func TestLoadConfigRootPath(t *testing.T) {
	t.Setenv("HOST_PROC", "testdata")
	factories, _ := otelcoltest.NopFactories()
	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33594
	// nolint:staticcheck
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-root-path.yaml"), factories)
	require.NoError(t, err)
	globalRootPath = ""

	r := cfg.Receivers[component.NewID(metadata.Type)].(*Config)
	expectedConfig := factory.CreateDefaultConfig().(*Config)
	expectedConfig.RootPath = "testdata"
	cpuScraperCfg := (&cpuscraper.Factory{}).CreateDefaultConfig()
	cpuScraperCfg.SetRootPath("testdata")
	cpuScraperCfg.SetEnvMap(common.EnvMap{
		common.HostDevEnvKey: "testdata/dev",
		common.HostEtcEnvKey: "testdata/etc",
		common.HostRunEnvKey: "testdata/run",
		common.HostSysEnvKey: "testdata/sys",
		common.HostVarEnvKey: "testdata/var",
	})
	expectedConfig.Scrapers = map[string]internal.Config{cpuscraper.TypeStr: cpuScraperCfg}

	assert.Equal(t, expectedConfig, r)
}

func TestLoadInvalidConfig_RootPathNotExist(t *testing.T) {
	factories, _ := otelcoltest.NopFactories()
	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33594
	// nolint:staticcheck
	_, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-bad-root-path.yaml"), factories)
	assert.ErrorContains(t, err, "invalid root_path:")
	globalRootPath = ""
}

func testValidate(rootPath string) error {
	err := validateRootPath(rootPath)
	globalRootPath = ""
	return err
}
