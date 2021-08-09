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

package podmanreceiver

import (
	"context"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
)

var _ component.MetricsReceiver = (*Receiver)(nil)

type Receiver struct {
	config       *Config
	logger       *zap.Logger
	nextConsumer consumer.Metrics
	obsrecv      *obsreport.Receiver
}

func NewReceiver(
	_ context.Context,
	logger *zap.Logger,
	config *Config,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	err := config.Validate()
	if err != nil {
		return nil, err
	}

	parsed, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("could not determine receiver transport: %w", err)
	}

	receiver := Receiver{
		config:       config,
		nextConsumer: nextConsumer,
		logger:       logger,
		obsrecv:      obsreport.NewReceiver(obsreport.ReceiverSettings{ReceiverID: config.ID(), Transport: parsed.Scheme}),
	}

	return &receiver, nil
}

func (r *Receiver) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (r *Receiver) Shutdown(ctx context.Context) error {
	return nil
}
