// Copyright 2019, OpenTelemetry Authors
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

package signalfxexporter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	otelconfig "go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

const (
	// The value of "type" key in configuration.
	typeStr = "signalfx"

	defaultHTTPTimeout = time.Second * 5
)

// NewFactory creates a factory for SignalFx exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithMetrics(createMetricsExporter),
		exporterhelper.WithLogs(createLogsExporter))
}

func createDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		Timeout: defaultHTTPTimeout,
		AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
			AccessTokenPassthrough: true,
		},
		SendCompatibleMetrics: false,
		TranslationRules:      nil,
		DeltaTranslationTTL:   3600,
	}
}

func createMetricsExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	config configmodels.Exporter,
) (exp component.MetricsExporter, err error) {

	expCfg := config.(*Config)
	if expCfg.SendCompatibleMetrics && expCfg.TranslationRules == nil {
		expCfg.TranslationRules, err = loadDefaultTranslationRules()
		if err != nil {
			return nil, err
		}
	}

	exp, err = newSignalFxExporter(expCfg, params.Logger)

	if err != nil {
		return nil, err
	}

	return exp, nil
}

func loadDefaultTranslationRules() ([]translation.Rule, error) {
	config := Config{}

	v := otelconfig.NewViper()
	v.SetConfigType("yaml")
	v.ReadConfig(strings.NewReader(translation.DefaultTranslationRulesYaml))
	err := v.UnmarshalExact(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to load default translation rules: %v", err)
	}

	return config.TranslationRules, nil
}

func createLogsExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.LogsExporter, error) {
	oCfg := cfg.(*Config)

	return NewEventExporter(oCfg, params.Logger)
}
