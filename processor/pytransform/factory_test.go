package pytransform

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.NotNil(t, cfg)
	assert.Equal(t, "send(event)", cfg.(*Config).Code)
}
