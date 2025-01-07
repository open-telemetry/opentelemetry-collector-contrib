package leaderelector

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/leaderelector/internal/metadata"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"testing"
	"time"
)

func TestNewFactory(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "creates a new factory with correct type",
			testFunc: func(t *testing.T) {
				t.Helper()
				factory := NewFactory()
				ft := factory.Type()
				require.EqualValues(t, metadata.Type, ft)
			},
		}, {
			desc: "creates a new factory and extension with default config",
			testFunc: func(t *testing.T) {
				t.Helper()
				factory := NewFactory()
				expectedCfg := &Config{
					LeaseDuration: 15 * time.Second,
					RenewDuration: 10 * time.Second,
					RetryPeriod:   2 * time.Second,
				}

				require.Equal(t, expectedCfg, factory.CreateDefaultConfig())
			},
		}, {
			desc: "creates a new factory and CreateExtension returns no error",
			testFunc: func(t *testing.T) {
				t.Helper()
				cfg := CreateDefaultConfig().(*Config)
				_, err := NewFactory().CreateExtension(
					context.Background(),
					extensiontest.NewNopSettings(),
					cfg,
				)
				require.NoError(t, err)
			},
		},
	}
	for _, test := range testCases {
		t.Run(test.desc, test.testFunc)
	}
}
