// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
)

func TestParseHostName(t *testing.T) {
	tmp := "mongodb://cluster0-shard-00-00.t5hdg.mongodb.net:27017,cluster0-shard-00-01.t5hdg.mongodb.net:27017,cluster0-shard-00-02.t5hdg.mongodb.net:27017/?ssl=true&authSource=admin&replicaSet=atlas-zx8u63-shard-0"
	hostnames := parseHostNames(tmp, zap.NewNop())
	require.Equal(t, []string{"cluster0-shard-00-00.t5hdg.mongodb.net", "cluster0-shard-00-01.t5hdg.mongodb.net", "cluster0-shard-00-02.t5hdg.mongodb.net"}, hostnames)
}

func TestFilterClusters(t *testing.T) {
	clusters := []mongodbatlas.Cluster{{Name: "cluster1", ID: "1"}, {Name: "cluster2", ID: "2"}, {Name: "cluster3", ID: "3"}}

	includeProject := ProjectConfig{
		IncludeClusters: []string{"cluster1", "cluster3"},
	}
	includeProject.populateIncludesAndExcludes()

	excludeProject := ProjectConfig{
		ExcludeClusters: []string{"cluster1", "cluster3"},
	}
	excludeProject.populateIncludesAndExcludes()

	ec, err := filterClusters(clusters, excludeProject)
	require.NoError(t, err)
	require.Equal(t, []mongodbatlas.Cluster{{Name: "cluster2", ID: "2"}}, ec)

	ic, err := filterClusters(clusters, includeProject)
	require.NoError(t, err)
	require.Equal(t, []mongodbatlas.Cluster{{Name: "cluster1", ID: "1"}, {Name: "cluster3", ID: "3"}}, ic)

}

func TestDefaultLoggingConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Logs.Enabled = true

	recv, err := createCombinedLogReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, recv, "receiver creation failed")

	err = recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestNoLoggingEnabled(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	recv, err := createCombinedLogReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.Error(t, err)
	require.Nil(t, recv, "receiver creation failed")
}
