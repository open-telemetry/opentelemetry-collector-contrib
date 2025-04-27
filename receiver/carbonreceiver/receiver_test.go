// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonreceiver

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/internal/client"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
)

func Test_carbonreceiver_New(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
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
			name: "default_config",
			args: args{
				config:       *defaultConfig,
				nextConsumer: consumertest.NewNop(),
			},
		},
		{
			name: "zero_value_parser",
			args: args{
				config: Config{
					AddrConfig: confignet.AddrConfig{
						Endpoint:  defaultConfig.Endpoint,
						Transport: defaultConfig.Transport,
					},
					TCPIdleTimeout: defaultConfig.TCPIdleTimeout,
				},
				nextConsumer: consumertest.NewNop(),
			},
		},
		{
			name: "empty_endpoint",
			args: args{
				config:       Config{},
				nextConsumer: consumertest.NewNop(),
			},
			wantErr: errEmptyEndpoint,
		},
		{
			name: "regex_parser",
			args: args{
				config: Config{
					AddrConfig: confignet.AddrConfig{
						Endpoint:  "localhost:2003",
						Transport: confignet.TransportTypeTCP,
					},
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
				nextConsumer: consumertest.NewNop(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newMetricsReceiver(receivertest.NewNopSettings(metadata.Type), tt.args.config, tt.args.nextConsumer)
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

func Test_carbonreceiver_Start(t *testing.T) {
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
			name: "invalid_transport",
			args: args{
				config: Config{
					AddrConfig: confignet.AddrConfig{
						Endpoint:  "localhost:2003",
						Transport: "unknown_transp",
					},
					Parser: &protocol.Config{
						Type:   "plaintext",
						Config: &protocol.PlaintextConfig{},
					},
				},
				nextConsumer: consumertest.NewNop(),
			},
			wantErr: errors.New("unsupported transport \"unknown_transp\""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newMetricsReceiver(receivertest.NewNopSettings(metadata.Type), tt.args.config, tt.args.nextConsumer)
			require.NoError(t, err)
			err = got.Start(context.Background(), componenttest.NewNopHost())
			assert.Equal(t, tt.wantErr, err)
			assert.NoError(t, got.Shutdown(context.Background()))
		})
	}
}

func Test_carbonreceiver_EndToEnd(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	tests := []struct {
		name     string
		configFn func() *Config
		clientFn func(t *testing.T) func(client.Metric) error
	}{
		{
			name: "default_config",
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
			clientFn: func(t *testing.T) func(client.Metric) error {
				c, err := client.NewGraphite(client.TCP, addr)
				require.NoError(t, err)
				return c.SendMetric
			},
		},
		{
			name: "tcp_reconnect",
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
			clientFn: func(t *testing.T) func(client.Metric) error {
				c, err := client.NewGraphite(client.TCP, addr)
				require.NoError(t, err)
				return c.SputterThenSendMetric
			},
		},
		{
			name: "default_config_udp",
			configFn: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Transport = confignet.TransportTypeUDP
				return cfg
			},
			clientFn: func(t *testing.T) func(client.Metric) error {
				c, err := client.NewGraphite(client.UDP, addr)
				require.NoError(t, err)
				return c.SendMetric
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.configFn()
			cfg.Endpoint = addr
			sink := new(consumertest.MetricsSink)
			recorder := tracetest.NewSpanRecorder()
			rt := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
			cs := receivertest.NewNopSettings(metadata.Type)
			cs.TracerProvider = rt
			rcv, err := newMetricsReceiver(cs, *cfg, sink)
			require.NoError(t, err)
			r := rcv.(*carbonReceiver)

			mr, err := newReporter(cs)
			require.NoError(t, err)
			r.reporter = mr

			host := &nopHost{
				reportFunc: func(event *componentstatus.Event) {
					assert.NoError(t, event.Err())
				},
			}

			require.NoError(t, r.Start(context.Background(), host))
			runtime.Gosched()
			defer func() {
				require.NoError(t, r.Shutdown(context.Background()))
			}()

			snd := tt.clientFn(t)

			ts := time.Now()
			carbonMetric := client.Metric{
				Name:      "tst_dbl",
				Value:     1.23,
				Timestamp: ts,
			}

			err = snd(carbonMetric)
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				return len(recorder.Ended()) == 1
			}, 30*time.Second, 100*time.Millisecond)

			mdd := sink.AllMetrics()
			require.Len(t, mdd, 1)
			require.Equal(t, 1, mdd[0].MetricCount())
			m := mdd[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
			assert.Equal(t, carbonMetric.Name, m.Name())
			require.Equal(t, 1, m.Gauge().DataPoints().Len())
			require.Len(t, recorder.Started(), len(recorder.Ended()))
		})
	}
}

var _ componentstatus.Reporter = (*nopHost)(nil)

type nopHost struct {
	reportFunc func(event *componentstatus.Event)
}

func (nh *nopHost) GetExtensions() map[component.ID]component.Component {
	return nil
}

func (nh *nopHost) Report(event *componentstatus.Event) {
	nh.reportFunc(event)
}
