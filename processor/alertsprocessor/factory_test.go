package alertsprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

func TestCreateDefaultConfig(t *testing.T) {
	c := createDefaultConfig()
	require.NotNil(t, c)
	cfg := c.(*Config)
	require.Equal(t, component.MustNewID(typeStr), cfg.ID())
	require.NoError(t, cfg.Validate())
}
