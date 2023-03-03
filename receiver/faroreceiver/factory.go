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

package faroreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
)

const (
	typeStr   = "faro"
	stability = component.StabilityLevelDevelopment

	defaultHTTPEndpoint = "0.0.0.0:8886"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithLogs(createLogReceiver, stability),
		receiver.WithTraces(createTraceReceiver, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTP: &confighttp.HTTPServerSettings{
			Endpoint: defaultHTTPEndpoint,
		},
	}
}

// createLogReceiver creates a log receiver based on provided config.
func createLogReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	next consumer.Logs,
) (receiver.Logs, error) {
	r := receivers.GetOrAdd(cfg, func() component.Component {
		c := cfg.(*Config)
		return newFaroReceiver(*c, set)
	})

	if err := r.Unwrap().(*faroReceiver).registerLogConsumer(set.Logger, next); err != nil {
		return nil, err
	}

	return r, nil
}

// createTraceReceiver creates a trace receiver based on provided config.
func createTraceReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	next consumer.Traces,
) (receiver.Traces, error) {
	r := receivers.GetOrAdd(cfg, func() component.Component {
		c := cfg.(*Config)
		return newFaroReceiver(*c, set)
	})

	if err := r.Unwrap().(*faroReceiver).registerTraceConsumer(set.Logger, next); err != nil {
		return nil, err
	}

	return r, nil
}

// This is the map of already created Faro receivers for particular configurations.
// We maintain this map because the Factory is asked trace and log receivers separately
// when it gets createTraceReceiver() and createLogReceiver() but they must not
// create separate objects, they must use one faroReceiver object per configuration.
var receivers = sharedcomponent.NewSharedComponents()
