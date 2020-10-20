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

package sapmexporter

import (
	"context"
	"time"

	"github.com/signalfx/signalfx-agent/pkg/apm/correlations"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

const (
	// The value of "type" key in configuration.
	typeStr = "sapm"
)

// NewFactory creates a factory for SAPM exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTraceExporter))
}

func createDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		NumWorkers: defaultNumWorkers,
		AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
			AccessTokenPassthrough: true,
		},
		TimeoutSettings: exporterhelper.CreateDefaultTimeoutSettings(),
		RetrySettings:   exporterhelper.CreateDefaultRetrySettings(),
		QueueSettings:   exporterhelper.CreateDefaultQueueSettings(),
		Correlation: CorrelationConfig{
			Enabled:             false,
			StaleServiceTimeout: 5 * time.Minute,
			SyncAttributes: map[string]string{
				conventions.AttributeK8sPodUID:   conventions.AttributeK8sPodUID,
				conventions.AttributeContainerID: conventions.AttributeContainerID,
			},
			Config: correlations.Config{
				MaxRequests:     20,
				MaxBuffered:     10_000,
				MaxRetries:      2,
				LogUpdates:      false,
				RetryDelay:      30 * time.Second,
				CleanupInterval: 1 * time.Minute,
			},
		},
	}
}

func createTraceExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.TraceExporter, error) {
	eCfg := cfg.(*Config)
	return newSAPMTraceExporter(eCfg, params)
}
