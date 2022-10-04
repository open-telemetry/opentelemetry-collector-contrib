package k8sobjectreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[config.Type(typeStr)] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Equal(t, len(cfg.Receivers), 1)

	r1 := cfg.Receivers[config.NewComponentID(typeStr)].(*Config)

	err = r1.Validate()
	require.Error(t, err)

	r1.makeDiscoveryClient = getMockDiscoveryClient

	expected := []*K8sObjectsConfig{
		{
			Name:          "pods",
			Mode:          PullMode,
			Interval:      time.Second * 30,
			FieldSelector: "status.phase=Running",
			LabelSelector: "environment in (production),tier in (frontend)",
		},
		{
			Name:       "events",
			Mode:       WatchMode,
			Namespaces: []string{"default"},
		},
	}
	assert.EqualValues(t, expected, r1.Objects)

	err = cfg.Validate()
	assert.NoError(t, err)

}

func TestValidConfigs(t *testing.T) {
	t.Parallel()
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[config.Type(typeStr)] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "invalid_config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	invalid_resource_config := cfg.Receivers[config.NewComponentIDWithName(typeStr, "invalid_resource")].(*Config)

	invalid_resource_config.makeDiscoveryClient = getMockDiscoveryClient

	err = invalid_resource_config.Validate()
	assert.ErrorContains(t, err, "resource fake_resource not found")

}
