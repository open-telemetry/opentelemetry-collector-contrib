// Copyright 2020 OpenTelemetry Authors
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

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
)

type receiver struct {
	config        *Config
	set           component.ReceiverCreateSettings
	clientFactory interface{}
	client        interface{}

	metricsComponent component.MetricsReceiver
	logsConsumer     consumer.Logs
	metricsConsumer  consumer.Metrics

	isLogsShutdown bool
	shutDownSync   sync.Mutex
}

func newReceiver(
	_ context.Context,
	settings component.ReceiverCreateSettings,
	config *Config,
	clientFactory interface{},
) (*receiver, error) {
	return nil, fmt.Errorf("podman receiver is not supported on windows")
}

func (r *receiver) registerMetricsConsumer(mc consumer.Metrics, set component.ReceiverCreateSettings) error {
	r.metricsConsumer = mc
	return nil
}

func (r *receiver) registerLogsConsumer(mc consumer.Logs) error {
	return nil
}

func (r *receiver) Shutdown(ctx context.Context) error {
	return nil
}

func (r *receiver) Start(ctx context.Context, host component.Host) error {
	return nil
}
