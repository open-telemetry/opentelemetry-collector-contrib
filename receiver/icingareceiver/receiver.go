// Copyright The OpenTelemetry Authors
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

package icingareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icingareceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

var _ component.MetricsReceiver = (*icingaReceiver)(nil)

// icingaReceiver implements the component.MetricsReceiver for Icigina event streams.
type icingaReceiver struct {
	settings component.ReceiverCreateSettings
	config   *Config

	client       icingaClient
	parser       icingaParser
	nextConsumer consumer.Metrics
	cancel       context.CancelFunc
}

// newIcingaReceiver creates the Icinga receiver with the given parameters.
func newIcingaReceiver(
	ctx context.Context,
	set component.ReceiverCreateSettings,
	config Config,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {

	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	if config.Host == "" {
		config.Host = "localhost:5665"
	}

	client, err := buildClient(config, set.Logger)
	if err != nil {
		return nil, err
	}

	r := &icingaReceiver{
		settings:     set,
		config:       &config,
		nextConsumer: nextConsumer,
		client:       client,
		parser:       icingaParser{ctx: ctx, config: config, logger: set.Logger},
	}
	return r, nil
}

func buildClient(config Config, logger *zap.Logger) (icingaClient, error) {
	return newClient(icingaClientConfig{
		Host:                   config.Host,
		Username:               config.Username,
		Password:               config.Password,
		DisableSslVerification: config.DisableSslVerification,
		Filter:                 config.Filter,
		logger:                 logger.Sugar(),
	})
}

// Start starts a client that can process Icinga event stream messages.
func (r *icingaReceiver) Start(ctx context.Context, host component.Host) error {
	ctx, r.cancel = context.WithCancel(ctx)

	r.parser.initialize()
	err := r.client.Listen(ctx, r.parser, r.nextConsumer)
	if err != nil {
		r.settings.Logger.Sugar().Error("Could not listen to Icinga", err)
		return err
	}

	return nil
}

// Shutdown stops the StatsD receiver.
func (r *icingaReceiver) Shutdown(context.Context) error {
	err := r.client.Close()
	r.cancel()
	return err
}
