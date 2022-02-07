// Copyright  The OpenTelemetry Authors
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

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
)

var (
	errUnrecognizedEncoding = fmt.Errorf("unrecognized encoding")
)

type firehoseMetricsReceiver struct {
	instanceID   config.ComponentID
	settings     component.ReceiverCreateSettings
	nextConsumer consumer.Metrics
	unmarshaler  MetricsUnmarshaler

	config *Config
}

var _ component.Receiver = (*firehoseMetricsReceiver)(nil)

func newMetricsReceiver(
	config *Config,
	set component.ReceiverCreateSettings,
	unmarshalers map[string]MetricsUnmarshaler,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	unmarshaler := unmarshalers[config.Encoding]
	if unmarshaler == nil {
		return nil, errUnrecognizedEncoding
	}

	return &firehoseMetricsReceiver{
		instanceID:   config.ID(),
		settings:     set,
		nextConsumer: nextConsumer,
		unmarshaler:  unmarshaler,
		config:       config,
	}, nil
}

func (fmr *firehoseMetricsReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (fmr *firehoseMetricsReceiver) Shutdown(context.Context) error {
	return nil
}
