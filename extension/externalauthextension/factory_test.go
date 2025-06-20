package externalauthextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestCreateExtension(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "http://example.com"

	ext, err := factory.Create(context.Background(), extensiontest.NewNopSettings(component.MustNewType("externalauth")), cfg)

	assert.NoError(t, err)
	assert.NotNil(t, ext)
}
