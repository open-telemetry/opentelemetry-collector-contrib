package activedirectorydsreceiver

import (
	"path/filepath"
	"testing"
	"time"

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
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Equal(t, len(cfg.Receivers), 2)
	defaultRecvID := config.NewComponentIDWithName(typeStr, "defaults")

	defaultCfg := factory.CreateDefaultConfig().(*Config)
	defaultCfg.ReceiverSettings.SetIDName(defaultRecvID.Name())
	defaultReceiver := cfg.Receivers[defaultRecvID]
	require.Equal(t, defaultCfg, defaultReceiver)

	advancedRecv := cfg.Receivers[config.NewComponentID(typeStr)]
	expectedAdvancedRecv := factory.CreateDefaultConfig().(*Config)

	expectedAdvancedRecv.Metrics.ActiveDirectoryDsReplicationObjectRate.Enabled = false
	expectedAdvancedRecv.ScraperControllerSettings.CollectionInterval = 2 * time.Minute

	require.Equal(t, expectedAdvancedRecv, advancedRecv)
}
