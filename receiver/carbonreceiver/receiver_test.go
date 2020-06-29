// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package carbonreceiver

import (
	"context"
	"errors"
	"net"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/testutil"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/transport"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/transport/client"
)

func Test_carbonreceiver_New(t *testing.T) {
	defaultConfig := (&Factory{}).CreateDefaultConfig().(*Config)
	type args struct {
		config       Config
		nextConsumer consumer.MetricsConsumerOld
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "default_config",
			args: args{
				config:       *defaultConfig,
				nextConsumer: new(exportertest.SinkMetricsExporterOld),
			},
		},
		{
			name: "zero_value_parser",
			args: args{
				config: Config{
					ReceiverSettings: defaultConfig.ReceiverSettings,
					Endpoint:         defaultConfig.Endpoint,
					Transport:        defaultConfig.Transport,
					TCPIdleTimeout:   defaultConfig.TCPIdleTimeout,
				},
				nextConsumer: new(exportertest.SinkMetricsExporterOld),
			},
		},
		{
			name: "nil_nextConsumer",
			args: args{
				config: *defaultConfig,
			},
			wantErr: errNilNextConsumer,
		},
		{
			name: "empty_endpoint",
			args: args{
				config: Config{
					ReceiverSettings: configmodels.ReceiverSettings{},
				},
				nextConsumer: new(exportertest.SinkMetricsExporterOld),
			},
			wantErr: errEmptyEndpoint,
		},
		{
			name: "invalid_transport",
			args: args{
				config: Config{
					ReceiverSettings: configmodels.ReceiverSettings{
						NameVal: "invalid_transport_rcv",
					},
					Endpoint:  "localhost:2003",
					Transport: "unknown_transp",
					Parser: &protocol.Config{
						Type:   "plaintext",
						Config: &protocol.PlaintextConfig{},
					},
				},
				nextConsumer: new(exportertest.SinkMetricsExporterOld),
			},
			wantErr: errors.New("unsupported transport \"unknown_transp\" for receiver \"invalid_transport_rcv\""),
		},
		{
			name: "regex_parser",
			args: args{
				config: Config{
					ReceiverSettings: configmodels.ReceiverSettings{
						NameVal: "regex_parser_rcv",
					},
					Endpoint:  "localhost:2003",
					Transport: "tcp",
					Parser: &protocol.Config{
						Type: "regex",
						Config: &protocol.RegexParserConfig{
							Rules: []*protocol.RegexRule{
								{
									Regexp: `(?P<key_root>[^.]*)\.test`,
								},
							},
						},
					},
				},
				nextConsumer: new(exportertest.SinkMetricsExporterOld),
			},
		},
		{
			name: "negative_tcp_idle_timeout",
			args: args{
				config: Config{
					ReceiverSettings: configmodels.ReceiverSettings{
						NameVal: "negative_tcp_idle_timeout",
					},
					Endpoint:       "localhost:2003",
					Transport:      "tcp",
					TCPIdleTimeout: -1 * time.Second,
					Parser: &protocol.Config{
						Type:   "plaintext",
						Config: &protocol.PlaintextConfig{},
					},
				},
				nextConsumer: new(exportertest.SinkMetricsExporterOld),
			},
			wantErr: errors.New("invalid idle timeout: -1s"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(zap.NewNop(), tt.args.config, tt.args.nextConsumer)
			assert.Equal(t, tt.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				assert.NoError(t, got.Shutdown(context.Background()))
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

func Test_carbonreceiver_EndToEnd(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	tests := []struct {
		name     string
		configFn func() *Config
		clientFn func(t *testing.T) *client.Graphite
	}{
		{
			name: "default_config",
			configFn: func() *Config {
				return (&Factory{}).CreateDefaultConfig().(*Config)
			},
			clientFn: func(t *testing.T) *client.Graphite {
				c, err := client.NewGraphite(client.TCP, host, port)
				require.NoError(t, err)
				return c
			},
		},
		{
			name: "default_config_udp",
			configFn: func() *Config {
				cfg := (&Factory{}).CreateDefaultConfig().(*Config)
				cfg.Transport = "udp"
				return cfg
			},
			clientFn: func(t *testing.T) *client.Graphite {
				c, err := client.NewGraphite(client.UDP, host, port)
				require.NoError(t, err)
				return c
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.configFn()
			cfg.Endpoint = addr
			sink := new(exportertest.SinkMetricsExporterOld)
			rcv, err := New(zap.NewNop(), *cfg, sink)
			require.NoError(t, err)
			r := rcv.(*carbonReceiver)

			mr := transport.NewMockReporter(1)
			r.reporter = mr

			require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
			runtime.Gosched()
			defer r.Shutdown(context.Background())
			require.Equal(t, componenterror.ErrAlreadyStarted, r.Start(context.Background(), componenttest.NewNopHost()))

			snd := tt.clientFn(t)

			ts := time.Now()
			carbonMetric := client.Metric{
				Name:      "tst_dbl",
				Value:     1.23,
				Timestamp: ts,
			}
			err = snd.SendMetric(carbonMetric)
			require.NoError(t, err)

			mr.WaitAllOnMetricsProcessedCalls()

			mdd := sink.AllMetrics()
			require.Equal(t, 1, len(mdd))
			require.Equal(t, 1, len(mdd[0].Metrics))
			metric := mdd[0].Metrics[0]
			assert.Equal(t, carbonMetric.Name, metric.GetMetricDescriptor().GetName())
			tss := metric.GetTimeseries()
			require.Equal(t, 1, len(tss))

			assert.NoError(t, r.Shutdown(context.Background()))
			assert.Equal(t, componenterror.ErrAlreadyStopped, r.Shutdown(context.Background()))
		})
	}
}
