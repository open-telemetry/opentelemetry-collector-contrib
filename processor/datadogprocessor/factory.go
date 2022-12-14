// Copyright The OpenTelemetry Authors
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

package datadogprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
)

// NewFactory creates a factory for the spanmetrics processor.
func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		"datadog",
		createDefaultConfig,
		component.WithTracesProcessor(createTracesProcessor, component.StabilityLevelBeta),
	)
}

func createTracesProcessor(ctx context.Context, params component.ProcessorCreateSettings, cfg component.Config, nextConsumer consumer.Traces) (component.TracesProcessor, error) {
	return newProcessor(ctx, params.Logger, cfg, nextConsumer)
}
