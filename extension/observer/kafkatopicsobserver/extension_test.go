// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatopicsobserver

import (
	"cmp"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
)

func TestStartShutdown(t *testing.T) {
	_, clientCfg := kafkatest.NewCluster(t)

	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.ClientConfig = clientCfg
	cfg.TopicRegex = ".*"

	ext, err := newObserver(zap.NewNop(), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)

	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, ext.Shutdown(t.Context()))
}

func TestObserveTopics(t *testing.T) {
	cluster, clientCfg := kafkatest.NewCluster(t,
		kfake.SeedTopics(1, "topic1", "topic2", "topics"),
	)

	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.ClientConfig = clientCfg
	cfg.TopicRegex = "^topic[0-9]$"
	cfg.TopicsSyncInterval = 100 * time.Millisecond

	ext, err := newObserver(zap.NewNop(), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)

	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, ext.Shutdown(t.Context()))
	})

	ops := make(channelNotifier, 1) // buffer for the initial op
	ext.(*kafkaTopicsObserver).ListAndWatch(ops)

	// First sync detects the two seeded topics matching the regex.
	assert.Equal(t, notifyOp{
		op: "add",
		endpoints: []observer.Endpoint{{
			ID:      "topic1",
			Target:  "topic1",
			Details: &observer.KafkaTopic{},
		}, {
			ID:      "topic2",
			Target:  "topic2",
			Details: &observer.KafkaTopic{},
		}},
	}, <-ops)

	// Delete topic2 from the cluster; the next sync should report its removal.
	adminCl, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	require.NoError(t, err)
	t.Cleanup(adminCl.Close)
	_, err = kadm.NewClient(adminCl).DeleteTopics(t.Context(), "topic2")
	require.NoError(t, err)

	assert.Equal(t, notifyOp{
		op: "remove",
		endpoints: []observer.Endpoint{{
			ID:      "topic2",
			Target:  "topic2",
			Details: &observer.KafkaTopic{},
		}},
	}, <-ops)
}

type channelNotifier chan notifyOp

func (channelNotifier) ID() observer.NotifyID {
	return "channel-notifier"
}

func (ch channelNotifier) OnAdd(added []observer.Endpoint) {
	ch.addOp("add", added)
}

func (ch channelNotifier) OnRemove(removed []observer.Endpoint) {
	ch.addOp("remove", removed)
}

func (ch channelNotifier) OnChange(changed []observer.Endpoint) {
	ch.addOp("remove", changed)
}

func (ch channelNotifier) addOp(op string, endpoints []observer.Endpoint) {
	ch <- notifyOp{
		op: op,
		endpoints: slices.SortedFunc(
			slices.Values(endpoints),
			func(a, b observer.Endpoint) int {
				return cmp.Compare(a.ID, b.ID)
			},
		),
	}
}

type notifyOp struct {
	op        string // add, remove, change
	endpoints []observer.Endpoint
}
