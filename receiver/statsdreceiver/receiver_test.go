// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsdreceiver

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport/client"
)

func Test_statsdreceiver_Start(t *testing.T) {
	type args struct {
		config       Config
		nextConsumer consumer.Metrics
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "unsupported transport",
			args: args{
				config: Config{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:8125",
						Transport: "unknown",
					},
				},
				nextConsumer: consumertest.NewNop(),
			},
			wantErr: errors.New("unsupported transport \"unknown\""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver, err := newReceiver(receivertest.NewNopCreateSettings(), tt.args.config, tt.args.nextConsumer)
			require.NoError(t, err)
			err = receiver.Start(context.Background(), componenttest.NewNopHost())
			assert.Equal(t, tt.wantErr, err)

			assert.NoError(t, receiver.Shutdown(context.Background()))
		})
	}
}

func TestStatsdReceiver_ShutdownBeforeStart(t *testing.T) {
	ctx := context.Background()
	cfg := createDefaultConfig().(*Config)
	nextConsumer := consumertest.NewNop()
	rcv, err := newReceiver(receivertest.NewNopCreateSettings(), *cfg, nextConsumer)
	assert.NoError(t, err)
	r := rcv.(*statsdReceiver)
	assert.NoError(t, r.Shutdown(ctx))
}

func TestStatsdReceiver_Flush(t *testing.T) {
	ctx := context.Background()
	cfg := createDefaultConfig().(*Config)
	nextConsumer := consumertest.NewNop()
	rcv, err := newReceiver(receivertest.NewNopCreateSettings(), *cfg, nextConsumer)
	assert.NoError(t, err)
	r := rcv.(*statsdReceiver)
	var metrics = pmetric.NewMetrics()
	assert.Nil(t, r.Flush(ctx, metrics, nextConsumer))
	assert.NoError(t, r.Start(ctx, componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(ctx))
}

func Test_statsdreceiver_EndToEnd(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		configFn func() *Config
		clientFn func(t *testing.T, addr string) *client.StatsD
	}{
		{
			name: "default_config with 4s interval",
			addr: testutil.GetAvailableLocalNetworkAddress(t, "udp"),
			configFn: func() *Config {
				return &Config{
					NetAddr: confignet.AddrConfig{
						Endpoint:  defaultBindEndpoint,
						Transport: confignet.TransportTypeUDP,
					},
					AggregationInterval: 4 * time.Second,
				}
			},
			clientFn: func(t *testing.T, addr string) *client.StatsD {
				c, err := client.NewStatsD("udp", addr)
				require.NoError(t, err)
				return c
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.configFn()
			cfg.NetAddr.Endpoint = tt.addr
			sink := new(consumertest.MetricsSink)
			rcv, err := newReceiver(receivertest.NewNopCreateSettings(), *cfg, sink)
			require.NoError(t, err)
			r := rcv.(*statsdReceiver)

			mr := transport.NewMockReporter(1)
			r.reporter = mr

			require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, r.Shutdown(context.Background()))
			}()

			statsdClient := tt.clientFn(t, tt.addr)

			statsdMetric := client.Metric{
				Name:  "test.metric",
				Value: "42",
				Type:  "c",
			}
			err = statsdClient.SendMetric(statsdMetric)
			require.NoError(t, err)

			time.Sleep(5 * time.Second)
			mdd := sink.AllMetrics()
			require.Len(t, mdd, 1)
			require.Equal(t, 1, mdd[0].ResourceMetrics().Len())
			require.Equal(t, 1, mdd[0].ResourceMetrics().At(0).ScopeMetrics().Len())
			require.Equal(t, 1, mdd[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
			metric := mdd[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
			assert.Equal(t, statsdMetric.Name, metric.Name())
			assert.Equal(t, pmetric.MetricTypeSum, metric.Type())
			require.Equal(t, 1, metric.Sum().DataPoints().Len())
			assert.NotEqual(t, 0, metric.Sum().DataPoints().At(0).Timestamp())
			assert.NotEqual(t, 0, metric.Sum().DataPoints().At(0).StartTimestamp())
			assert.Less(t, metric.Sum().DataPoints().At(0).StartTimestamp(), metric.Sum().DataPoints().At(0).Timestamp())

			// Send the same metric again to ensure that the timestamps of successive data points
			// are aligned.
			statsdMetric.Value = "43"
			err = statsdClient.SendMetric(statsdMetric)
			require.NoError(t, err)

			time.Sleep(5 * time.Second)
			mddAfter := sink.AllMetrics()
			require.Len(t, mddAfter, 2)
			metricAfter := mddAfter[1].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
			require.Equal(t, metric.Sum().DataPoints().At(0).Timestamp(), metricAfter.Sum().DataPoints().At(0).StartTimestamp())
		})
	}
}

func Test_statsdreceiver_resource_attribute_source(t *testing.T) {
	serverAddr := testutil.GetAvailableLocalNetworkAddress(t, "udp")
	firstStatsdClient, err := client.NewStatsD("udp", serverAddr)
	require.NoError(t, err)
	secondStatsdClient, err := client.NewStatsD("udp", serverAddr)
	require.NoError(t, err)
	t.Run("aggregate by source address with two clients sending", func(t *testing.T) {
		cfg := &Config{
			NetAddr: confignet.AddrConfig{
				Endpoint:  serverAddr,
				Transport: confignet.TransportTypeUDP,
			},
			AggregationInterval: 4 * time.Second,
			AggregateBySourceAddr: true,
		}
		sink := new(consumertest.MetricsSink)
		rcv, err := newReceiver(receivertest.NewNopCreateSettings(), *cfg, sink)
		require.NoError(t, err)
		r := rcv.(*statsdReceiver)

		mr := transport.NewMockReporter(1)
		r.reporter = mr

		require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
		defer func() {
			assert.NoError(t, r.Shutdown(context.Background()))
		}()

		statsdMetric := client.Metric{
			Name:  "test.metric",
			Value: "42",
			Type:  "c",
		}
		err = firstStatsdClient.SendMetric(statsdMetric)
		require.NoError(t, err)
		err = secondStatsdClient.SendMetric(statsdMetric)
		require.NoError(t, err)

		// Wait for aggregation interval
		time.Sleep(5 * time.Second)

		// We should have two resources which one metric each
		mdd := sink.AllMetrics()
		require.Len(t, mdd, 2)
		require.Equal(t, 1, mdd[0].ResourceMetrics().Len())
		require.Equal(t, 1, mdd[1].ResourceMetrics().Len())

		// The resources should have source attributes matching the client addr
		firstResource := mdd[0].ResourceMetrics().At(0)
		sourceAddress, exists := firstResource.Resource().Attributes().Get("source.address")
		assert.Equal(t, true, exists)
		sourcePort, exists := firstResource.Resource().Attributes().Get("source.port")
		assert.Equal(t, true, exists)
		clientAddress, clientPort, err := net.SplitHostPort(firstStatsdClient.Conn.LocalAddr().String())
		require.NoError(t, err)
		assert.Equal(t, clientAddress, sourceAddress.AsString())
		assert.Equal(t, clientPort, sourcePort.AsString())

		secondResource := mdd[1].ResourceMetrics().At(0)
		sourceAddress, exists = secondResource.Resource().Attributes().Get("source.address")
		assert.Equal(t, true, exists)
		sourcePort, exists = secondResource.Resource().Attributes().Get("source.port")
		assert.Equal(t, true, exists)
		clientAddress, clientPort, err = net.SplitHostPort(secondStatsdClient.Conn.LocalAddr().String())
		require.NoError(t, err)
		assert.Equal(t, clientAddress, sourceAddress.AsString())
		assert.Equal(t, clientPort, sourcePort.AsString())
	})
	t.Run("do not aggregate by source address with two clients sending", func(t *testing.T) {
		cfg := &Config{
			NetAddr: confignet.AddrConfig{
				Endpoint:  serverAddr,
				Transport: confignet.TransportTypeUDP,
			},
			AggregationInterval: 4 * time.Second,
			AggregateBySourceAddr: false,
		}
		sink := new(consumertest.MetricsSink)
		rcv, err := newReceiver(receivertest.NewNopCreateSettings(), *cfg, sink)
		require.NoError(t, err)
		r := rcv.(*statsdReceiver)

		mr := transport.NewMockReporter(1)
		r.reporter = mr

		require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
		defer func() {
			assert.NoError(t, r.Shutdown(context.Background()))
		}()

		statsdMetric := client.Metric{
			Name:  "test.metric",
			Value: "42",
			Type:  "c",
		}
		err = firstStatsdClient.SendMetric(statsdMetric)
		require.NoError(t, err)
		err = secondStatsdClient.SendMetric(statsdMetric)
		require.NoError(t, err)

		// Wait for aggregation interval
		time.Sleep(5 * time.Second)

		// We should have one resource with one metric due to disabled aggregation by source address
		mdd := sink.AllMetrics()
		require.Len(t, mdd, 1)
		require.Equal(t, 1, mdd[0].ResourceMetrics().Len())

		// The resources should have source attribute matching the servers address
		resource := mdd[0].ResourceMetrics().At(0)
		sourceAddress, exists := resource.Resource().Attributes().Get("source.address")
		assert.Equal(t, true, exists)
		sourcePort, exists := resource.Resource().Attributes().Get("source.port")
		assert.Equal(t, true, exists)
		serverAddr, serverPort, err := net.SplitHostPort(serverAddr)
		require.NoError(t, err)
		assert.Equal(t, serverAddr, sourceAddress.AsString())
		assert.Equal(t, serverPort, sourcePort.AsString())
	})
}
