// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lokireceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/lokireceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver"
)

type lokiReceiver struct {
	conf         *Config
	nextConsumer consumer.Logs
	settings     receiver.CreateSettings

	obsrepGRPC *obsreport.Receiver
	obsrepHTTP *obsreport.Receiver
}

func newLokiReceiver(conf *Config, nextConsumer consumer.Logs, settings receiver.CreateSettings) (*lokiReceiver, error) {
	r := &lokiReceiver{
		conf:         conf,
		nextConsumer: nextConsumer,
		settings:     settings,
	}

	var err error
	r.obsrepGRPC, err = obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             settings.ID,
		Transport:              "grpc",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	r.obsrepHTTP, err = obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             settings.ID,
		Transport:              "http",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	return r, err
}

func (r *lokiReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (r *lokiReceiver) Shutdown(_ context.Context) error {
	return nil
}
