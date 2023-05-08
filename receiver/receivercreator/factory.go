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

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// This file implements factory for receiver_creator. A receiver_creator can create other receivers at runtime.

const (
	typeStr   = "receiver_creator"
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for receiver creator.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability))
}

func createDefaultConfig() component.Config {
	return &Config{
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
			observer.ContainerType: map[string]string{
				conventions.AttributeContainerName:      "`name`",
				conventions.AttributeContainerImageName: "`image`",
			},
			observer.K8sNodeType: map[string]string{
				conventions.AttributeK8SNodeName: "`name`",
				conventions.AttributeK8SNodeUID:  "`uid`",
			},
		},
		receiverTemplates: map[string]receiverTemplate{},
	}
}

func createMetricsReceiver(
	ctx context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	return newReceiverCreator(params, cfg.(*Config), consumer)
}
