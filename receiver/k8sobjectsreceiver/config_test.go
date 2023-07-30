// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	sub, err := cm.Sub("k8sobjects")
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))
	require.NotNil(t, cfg)

	err = component.ValidateConfig(cfg)
	require.Error(t, err)

	cfg.makeDiscoveryClient = getMockDiscoveryClient

	err = component.ValidateConfig(cfg)
	require.NoError(t, err)

	expected := []*K8sObjectsConfig{
		{
			Name:          "pods",
			Mode:          PullMode,
			Interval:      time.Hour,
			FieldSelector: "status.phase=Running",
			LabelSelector: "environment in (production),tier in (frontend)",
			gvr: &schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
		},
		{
			Name:            "events",
			Mode:            WatchMode,
			Namespaces:      []string{"default"},
			Group:           "events.k8s.io",
			ResourceVersion: "1",
			gvr: &schema.GroupVersionResource{
				Group:    "events.k8s.io",
				Version:  "v1",
				Resource: "events",
			},
		},
	}
	assert.EqualValues(t, expected, cfg.Objects)

}

func TestValidConfigs(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid_config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	sub, err := cm.Sub("k8sobjects/invalid_resource")
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	cfg.makeDiscoveryClient = getMockDiscoveryClient

	err = component.ValidateConfig(cfg)
	assert.ErrorContains(t, err, "resource fake_resource not found")

}

func TestValidateResourceConflict(t *testing.T) {
	t.Parallel()

	mockClient := newMockDynamicClient()
	rCfg := createDefaultConfig().(*Config)
	rCfg.makeDynamicClient = mockClient.getMockDynamicClient
	rCfg.makeDiscoveryClient = getMockDiscoveryClient

	// Validate it should choose first gvr if group is not specified
	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name: "myresources",
			Mode: PullMode,
		},
	}

	err := rCfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, "group1", rCfg.Objects[0].gvr.Group)

	// Validate it should choose gvr for specified group
	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name:  "myresources",
			Mode:  PullMode,
			Group: "group2",
		},
	}

	err = rCfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, "group2", rCfg.Objects[0].gvr.Group)
}

func TestPullResourceVersion(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "pull_resource_version_config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	sub, err := cm.Sub("k8sobjects")
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))
	require.NotNil(t, cfg)

	err = component.ValidateConfig(cfg)
	require.Error(t, err)

	require.Equal(t, "1", cfg.Objects[0].ResourceVersion)
	require.Equal(t, "", cfg.Objects[1].ResourceVersion)
}

func TestWatchResourceVersion(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_watch_resource_version.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	sub, err := cm.Sub("k8sobjects")
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))
	require.NotNil(t, cfg)

	err = component.ValidateConfig(cfg)
	require.Error(t, err)

	cfg.makeDiscoveryClient = getMockDiscoveryClient

	err = component.ValidateConfig(cfg)
	require.NoError(t, err)

	expected := []*K8sObjectsConfig{
		{
			Name:            "events",
			Mode:            WatchMode,
			Namespaces:      []string{"default"},
			Group:           "events.k8s.io",
			ResourceVersion: "1",
			gvr: &schema.GroupVersionResource{
				Group:    "events.k8s.io",
				Version:  "v1",
				Resource: "events",
			},
		},
		{
			Name:            "events",
			Mode:            WatchMode,
			Namespaces:      []string{"default"},
			Group:           "events.k8s.io",
			ResourceVersion: "2",
			gvr: &schema.GroupVersionResource{
				Group:    "events.k8s.io",
				Version:  "v1",
				Resource: "events",
			},
		},
	}
	assert.EqualValues(t, expected, cfg.Objects)

}
