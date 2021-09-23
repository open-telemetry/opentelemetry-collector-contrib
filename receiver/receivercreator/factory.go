// Copyright 2020, OpenTelemetry Authors
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

package receivercreator

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// This file implements factory for receiver_creator. A receiver_creator can create other receivers at runtime.

const (
	typeStr = "receiver_creator"
)

// NewFactory creates a factory for receiver creator.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewID(typeStr)),
		ResourceAttributes: resourceAttributes{
			observer.PodType: map[string]string{
				conventions.AttributeK8SPodName:       "`name`",
				conventions.AttributeK8SPodUID:        "`uid`",
				conventions.AttributeK8SNamespaceName: "`namespace`",
			},
			observer.PortType: map[string]string{
				conventions.AttributeK8SPodName:       "`pod.name`",
				conventions.AttributeK8SPodUID:        "`pod.uid`",
				conventions.AttributeK8SNamespaceName: "`pod.namespace`",
			},
		},
		receiverTemplates: map[string]receiverTemplate{},
	}
}

func createMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateSettings,
	cfg config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	return newReceiverCreator(params, cfg.(*Config), consumer)
}
