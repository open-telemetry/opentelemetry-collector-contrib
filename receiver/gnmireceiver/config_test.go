package gnmireceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewID(factory.Type()).String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	expected := &Config{
		Targets: []TargetConfig{
			{
				Address:  "10.0.0.1:57400",
				Username: "admin",
				Password: "password",
				Encoding: "proto",
				Redial:   10 * time.Second,
				Subscriptions: []SubscriptionConfig{
					{
						Name:             "interfaces",
						Path:             "/interfaces/interface/state/counters",
						SubscriptionMode: "sample",
						SampleInterval:   30 * time.Second,
					},
				},
			},
		},
	}
	assert.Equal(t, expected, cfg)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "Empty targets",
			cfg:     &Config{Targets: []TargetConfig{}},
			wantErr: true,
		},
		{
			name: "Invalid Redial",
			cfg: &Config{
				Targets: []TargetConfig{
					{Address: "localhost:5000", Redial: 500 * time.Millisecond},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
