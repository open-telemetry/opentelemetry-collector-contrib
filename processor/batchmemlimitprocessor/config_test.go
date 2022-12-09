package batchmemlimitprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestConfigLoad(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	assert.Nil(t, err)
	require.NotNil(t, cfg)

	id := component.NewIDWithName(typeStr, "")
	component, ok := cfg.Processors[id]
	assert.True(t, ok)
	assert.Equal(t, component.(*Config).SendMemorySize, uint32(128))

}
