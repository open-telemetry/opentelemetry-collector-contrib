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
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"
)

func TestLoadConfigRootPath(t *testing.T) {
	t.Cleanup(func() { gopsutilenv.SetGlobalRootPath("") })
	t.Setenv("HOST_PROC", "testdata")
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config-root-path.yaml"))
	require.NoError(t, err)
	require.NoError(t, cm.Unmarshal(cfg))

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
	envMap := gopsutilenv.SetGoPsutilEnvVars("testdata")
	require.NoError(t, err)
	assert.Equal(t, expectedEnvMap, envMap)
}

func TestLoadInvalidConfig_RootPathNotExist(t *testing.T) {
	t.Cleanup(func() { gopsutilenv.SetGlobalRootPath("") })
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config-bad-root-path.yaml"))
	require.NoError(t, err)
	require.NoError(t, cm.Unmarshal(cfg))
	assert.ErrorContains(t, xconfmap.Validate(cfg), "invalid root_path:")
	t.Cleanup(func() { gopsutilenv.SetGlobalRootPath("") })
}

func TestLoadConfigRootPath_InvalidConfig_InconsistentRootPaths(t *testing.T) {
	t.Cleanup(func() { gopsutilenv.SetGlobalRootPath("") })
	gopsutilenv.SetGlobalRootPath("foo")
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config-root-path.yaml"))
	require.NoError(t, err)
	require.NoError(t, cm.Unmarshal(cfg))
	assert.EqualError(t, xconfmap.Validate(cfg), "inconsistent root_path configuration detected among components: `foo` != `testdata`")
}
