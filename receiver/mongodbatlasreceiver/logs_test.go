// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongodbatlasreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

func TestParseHostName(t *testing.T) {
	tmp := "mongodb://cluster0-shard-00-00.t5hdg.mongodb.net:27017,cluster0-shard-00-01.t5hdg.mongodb.net:27017,cluster0-shard-00-02.t5hdg.mongodb.net:27017/?ssl=true&authSource=admin&replicaSet=atlas-zx8u63-shard-0"
	hostnames := parseHostNames(tmp, zap.NewNop())
	require.Equal(t, []string{"cluster0-shard-00-00.t5hdg.mongodb.net", "cluster0-shard-00-01.t5hdg.mongodb.net", "cluster0-shard-00-02.t5hdg.mongodb.net"}, hostnames)
}

func TestFilterClusters(t *testing.T) {
	clusters := []mongodbatlas.Cluster{{Name: "cluster1", ID: "1"}, {Name: "cluster2", ID: "2"}, {Name: "cluster3", ID: "3"}}

	exclude := []string{"cluster1", "cluster3"}
	include := []string{"cluster1", "cluster3"}
	ec := filterClusters(clusters, exclude, false)
	require.Equal(t, []mongodbatlas.Cluster{{Name: "cluster2", ID: "2"}}, ec)

	ic := filterClusters(clusters, include, true)
	require.Equal(t, []mongodbatlas.Cluster{{Name: "cluster1", ID: "1"}, {Name: "cluster3", ID: "3"}}, ic)

}

func TestDefaultLoggingConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.SetIDName("LoggingID")
	params := componenttest.NewNopReceiverCreateSettings()
	ctx := context.Background()
	cfg.Logs.Enabled = true

	receiver, err := createCombinedLogReceiver(
		ctx,
		params,
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, receiver, "receiver creation failed")

	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	err = receiver.Shutdown(ctx)
	require.NoError(t, err)
}

func TestNoLoggingEnabled(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.SetIDName("LoggingID")
	params := componenttest.NewNopReceiverCreateSettings()
	ctx := context.Background()

	receiver, err := createCombinedLogReceiver(
		ctx,
		params,
		cfg,
		consumertest.NewNop(),
	)
	require.Error(t, err)
	require.Nil(t, receiver, "receiver creation failed")
}
