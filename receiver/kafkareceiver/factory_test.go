// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/custombalancer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

func encodingFromReceiver(tb testing.TB, r any, section string) string {
	tb.Helper()

	if rc, ok := r.(*franzConsumer); ok {
		switch section {
		case "Traces":
			return rc.config.Traces.Encoding
		case "Metrics":
			return rc.config.Metrics.Encoding
		case "Logs":
			return rc.config.Logs.Encoding
		case "Profiles":
			return rc.config.Profiles.Encoding
		}
	}

	tb.Fatalf("unsupported receiver type %T or section %q", r, section)
	return ""
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, configkafka.NewDefaultClientConfig(), cfg.ClientConfig)
	assert.Equal(t, configkafka.NewDefaultConsumerConfig(), cfg.ConsumerConfig)
}

func TestNewFactory_UsesRegisteredGroupBalancerResolver(t *testing.T) {
	var resolverCalled bool
	resolver := func(strategy string) ([]kgo.GroupBalancer, bool, error) {
		resolverCalled = true
		if strategy == "" {
			return []kgo.GroupBalancer{kgo.RangeBalancer()}, true, nil
		}
		return nil, false, nil
	}

	base := NewFactory()
	f := custombalancer.WrapFactory(base, resolver)

	cfg := createDefaultConfig().(*Config)
	cfg.GroupRebalanceStrategy = ""

	r, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)

	c, ok := r.(*franzConsumer)
	require.True(t, ok)
	require.NotNil(t, c.groupBalancerResolver)

	balancers, handled, err := c.groupBalancerResolver("")
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, balancers, 1)
	assert.True(t, resolverCalled)
}

func TestNewFactory_WrappedFactoryStart_UsesCustomBalancerWhenUnset(t *testing.T) {
	t.Parallel()

	cluster, clientConfig := kafkatest.NewCluster(t, kfake.SeedTopics(1, defaultLogsTopic))
	producer := mustNewClient(t, cluster)

	seenProtocols := make(chan []string, 1)
	cluster.ControlKey(int16(kmsg.JoinGroup), func(kreq kmsg.Request) (kmsg.Response, error, bool) {
		jreq, ok := kreq.(*kmsg.JoinGroupRequest)
		require.True(t, ok)

		names := make([]string, 0, len(jreq.Protocols))
		for _, p := range jreq.Protocols {
			names = append(names, p.Name)
		}
		select {
		case seenProtocols <- names:
		default:
		}
		return nil, nil, false
	})

	customProtocol := "private-coop-sticky"
	var resolverCalled bool
	resolver := func(strategy string) ([]kgo.GroupBalancer, bool, error) {
		resolverCalled = true
		require.Zero(t, strategy)
		return []kgo.GroupBalancer{
			namedTestBalancer{
				GroupBalancer: kgo.CooperativeStickyBalancer(),
				name:          customProtocol,
			},
		}, true, nil
	}

	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig = clientConfig
	cfg.GroupID = t.Name()
	cfg.InitialOffset = configkafka.EarliestOffset
	cfg.GroupRebalanceStrategy = ""

	set, _, _ := mustNewSettings(t)
	var sink consumertest.LogsSink
	f := custombalancer.WrapFactory(NewFactory(), resolver)
	r, err := f.CreateLogs(t.Context(), set, cfg, &sink)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting

	logs := testdata.GenerateLogs(1)
	data, err := (&plog.ProtoMarshaler{}).MarshalLogs(logs)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	require.NoError(t, producer.ProduceSync(ctx, &kgo.Record{
		Topic: defaultLogsTopic,
		Value: data,
	}).FirstErr())

	assert.Eventually(t, func() bool {
		return len(sink.AllLogs()) == 1
	}, 2*time.Second, 20*time.Millisecond)
	assert.True(t, resolverCalled)

	select {
	case names := <-seenProtocols:
		assert.Contains(t, names, customProtocol)
	case <-ctx.Done():
		t.Fatal("timeout waiting for JoinGroup request")
	}
}

func TestNewFactory_WrappedFactoryStart_ConfiguredStrategySkipsCustomResolver(t *testing.T) {
	t.Parallel()

	cluster, clientConfig := kafkatest.NewCluster(t, kfake.SeedTopics(1, defaultLogsTopic))
	producer := mustNewClient(t, cluster)

	seenProtocols := make(chan []string, 1)
	cluster.ControlKey(int16(kmsg.JoinGroup), func(kreq kmsg.Request) (kmsg.Response, error, bool) {
		jreq, ok := kreq.(*kmsg.JoinGroupRequest)
		require.True(t, ok)

		names := make([]string, 0, len(jreq.Protocols))
		for _, p := range jreq.Protocols {
			names = append(names, p.Name)
		}
		select {
		case seenProtocols <- names:
		default:
		}
		return nil, nil, false
	})

	var resolverCalled bool
	resolver := func(string) ([]kgo.GroupBalancer, bool, error) {
		resolverCalled = true
		return []kgo.GroupBalancer{
			namedTestBalancer{
				GroupBalancer: kgo.CooperativeStickyBalancer(),
				name:          "private-coop-sticky",
			},
		}, true, nil
	}

	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig = clientConfig
	cfg.GroupID = t.Name()
	cfg.InitialOffset = configkafka.EarliestOffset
	cfg.GroupRebalanceStrategy = configkafka.StickyBalanceStrategy

	set, _, _ := mustNewSettings(t)
	var sink consumertest.LogsSink
	f := custombalancer.WrapFactory(NewFactory(), resolver)
	r, err := f.CreateLogs(t.Context(), set, cfg, &sink)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting

	logs := testdata.GenerateLogs(1)
	data, err := (&plog.ProtoMarshaler{}).MarshalLogs(logs)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	require.NoError(t, producer.ProduceSync(ctx, &kgo.Record{
		Topic: defaultLogsTopic,
		Value: data,
	}).FirstErr())

	assert.Eventually(t, func() bool {
		return len(sink.AllLogs()) == 1
	}, 2*time.Second, 20*time.Millisecond)
	assert.False(t, resolverCalled)

	select {
	case names := <-seenProtocols:
		assert.Contains(t, names, string(configkafka.StickyBalanceStrategy))
		assert.NotContains(t, names, "private-coop-sticky")
	case <-ctx.Done():
		t.Fatal("timeout waiting for JoinGroup request")
	}
}

func TestCreateTraces(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"localhost:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createTracesReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(t.Context()))
}

func TestWithTracesUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Traces.Encoding = "custom"
		receiver, err := f.CreateTraces(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Traces"))
	})

	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		receiver, err := f.CreateTraces(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultTracesEncoding, encodingFromReceiver(t, receiver, "Traces"))
	})
}

func TestCreateMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"localhost:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createMetricsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(t.Context()))
}

func TestWithMetricsUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Metrics.Encoding = "custom"
		receiver, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Metrics"))
	})

	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		receiver, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultMetricsEncoding, encodingFromReceiver(t, receiver, "Metrics"))
	})
}

func TestCreateLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"localhost:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createLogsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(t.Context()))
}

func TestWithLogsUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Logs.Encoding = "custom"
		receiver, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Logs"))
	})

	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		receiver, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultLogsEncoding, encodingFromReceiver(t, receiver, "Logs"))
	})
}

func TestCreateProfiles(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"localhost:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createProfilesReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(t.Context()))
}

func TestWithProfilesUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Profiles.Encoding = "custom"
		receiver, err := f.(xreceiver.Factory).CreateProfiles(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Profiles"))
	})

	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		receiver, err := f.(xreceiver.Factory).CreateProfiles(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultProfilesEncoding, encodingFromReceiver(t, receiver, "Profiles"))
	})
}
