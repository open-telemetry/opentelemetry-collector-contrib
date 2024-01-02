// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.Equal(t, cfg.(*Config).ScraperControllerSettings.CollectionInterval, 3*time.Minute)
	recv, err := createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, recv, "receiver creation failed")

	err = recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestTimeConstraints(t *testing.T) {
	tt := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "initial lookback is now() - collection_interval",
			run: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig().(*Config)
				// lastRun is nil
				recv := mongodbatlasreceiver{
					cfg: cfg,
				}
				now := time.Now()
				tc := recv.timeConstraints(now)
				require.NotNil(t, tc)
				require.Equal(t, tc.start, now.Add(cfg.CollectionInterval*-1).UTC().Format(time.RFC3339))
			},
		},
		{
			name: "lookback for subsequent runs is now() - lastRun",
			run: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig().(*Config)
				now := time.Now()
				recv := mongodbatlasreceiver{
					cfg: cfg,
					// set last run to 1 collection ago
					lastRun: now.Add(cfg.CollectionInterval * -1),
				}
				tc := recv.timeConstraints(now)
				require.NotNil(t, tc)
				require.Equal(t, tc.start, recv.lastRun.UTC().Format(time.RFC3339))
			},
		},
	}

	for _, testCase := range tt {
		t.Run(testCase.name, testCase.run)
	}
}

func TestShouldProcessCluster(t *testing.T) {
	tests := []struct {
		name        string
		projectCfg  *ProjectConfig
		clusterName string
		want        bool
	}{
		{
			name: "included cluster should be processed",
			projectCfg: &ProjectConfig{
				IncludeClusters: []string{"Cluster1"},
			},
			clusterName: "Cluster1",
			want:        true,
		},
		{
			name: "cluster not included should not be processed",
			projectCfg: &ProjectConfig{
				IncludeClusters: []string{"Cluster1"},
			},
			clusterName: "Cluster2",
			want:        false,
		},
		{
			name: "excluded cluster should not be processed",
			projectCfg: &ProjectConfig{
				ExcludeClusters: []string{"Cluster2"},
			},
			clusterName: "Cluster2",
			want:        false,
		},
		{
			name: "cluster not excluded should processed assuming it exists in the project",
			projectCfg: &ProjectConfig{
				ExcludeClusters: []string{"Cluster1"},
			},
			clusterName: "Cluster2",
			want:        true,
		},
		{
			name:        "cluster should be processed when no includes or excludes are set",
			projectCfg:  &ProjectConfig{},
			clusterName: "Cluster1",
			want:        true,
		},
		{
			name:        "cluster should be processed when no includes or excludes are set and cluster name is empty",
			projectCfg:  nil,
			clusterName: "Cluster1",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.projectCfg != nil {
				tt.projectCfg.populateIncludesAndExcludes()
			}

			if got := shouldProcessCluster(tt.projectCfg, tt.clusterName); got != tt.want {
				t.Errorf("shouldProcessCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}
