package leaderelector

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/leaderelector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id             component.ID
		expectedConfig component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "foo"),
			expectedConfig: &Config{
				APIConfig: k8sconfig.APIConfig{
					AuthType: "kubeConfig",
				},
				LeaseName:      "foo",
				LeaseNamespace: "default",
				LeaseDuration:  15 * time.Second,
				RenewDuration:  10 * time.Second,
				RetryPeriod:    2 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "bar"),
			expectedConfig: &Config{
				APIConfig: k8sconfig.APIConfig{
					AuthType: "kubeConfig",
				},
				LeaseName:      "bar",
				LeaseNamespace: "default",
				LeaseDuration:  20 * time.Second,
				RenewDuration:  10 * time.Second,
				RetryPeriod:    2 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			require.Equal(t, tt.expectedConfig, cfg)
		})
	}

}
