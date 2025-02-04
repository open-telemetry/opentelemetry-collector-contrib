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
	"go.opentelemetry.io/collector/confmap/confmaptest"

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
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config-root-path.yaml"))
	require.NoError(t, err)
	require.NoError(t, cm.Unmarshal(cfg))

	globalRootPath = ""

	expectedConfig := factory.CreateDefaultConfig().(*Config)
	expectedConfig.RootPath = "testdata"
	f := cpuscraper.NewFactory()
	cpuScraperCfg := f.CreateDefaultConfig()
	expectedConfig.Scrapers = map[component.Type]component.Config{f.Type(): cpuScraperCfg}
	assert.Equal(t, expectedConfig, cfg)
	expectedEnvMap := common.EnvMap{
		common.HostDevEnvKey: "testdata/dev",
		common.HostEtcEnvKey: "testdata/etc",
		common.HostRunEnvKey: "testdata/run",
		common.HostSysEnvKey: "testdata/sys",
		common.HostVarEnvKey: "testdata/var",
	}
	assert.Equal(t, expectedEnvMap, setGoPsutilEnvVars("testdata"))
}

func TestLoadInvalidConfig_RootPathNotExist(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config-bad-root-path.yaml"))
	require.NoError(t, err)
	require.NoError(t, cm.Unmarshal(cfg))
	assert.ErrorContains(t, component.ValidateConfig(cfg), "invalid root_path:")
	globalRootPath = ""
}

func testValidate(rootPath string) error {
	err := validateRootPath(rootPath)
	globalRootPath = ""
	return err
}
