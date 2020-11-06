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

package awsprometheusremotewriteexporter

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	prw "go.opentelemetry.io/collector/exporter/prometheusremotewriteexporter"
)

const (
	typeStr = "awsprometheusremotewrite" // The value of "type" key in configuration.
)

// NewFactory returns a factory of the AWS Prometheus Remote Write exporter that can be registered to the Collector.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithMetrics(createMetricsExporter))
}

func createMetricsExporter(_ context.Context, params component.ExporterCreateParams,
	cfg configmodels.Exporter) (component.MetricsExporter, error) {
	// check if the configuration is valid
	prwCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration")
	}

	if !validateAuthConfig(prwCfg.AuthSettings) {
		return nil, errors.New("invalid authentication configuration")
	}

	client, cerr := prwCfg.HTTPClientSettings.ToClient()
	if cerr != nil {
		return nil, cerr
	}

	// load AWS auth configurations and create interceptor based on configuration
	if applyAuth(prwCfg.AuthSettings) {
		roundTripper, err := NewAuth(prwCfg.AuthSettings, client)
		if err != nil {
			return nil, err
		}
		client.Transport = roundTripper
	}

	// initialize an upstream exporter and pass it an http.Client with interceptor
	prwe, err := prw.NewPrwExporter(prwCfg.Namespace, prwCfg.HTTPClientSettings.Endpoint, client, prwCfg.ExternalLabels)
	if err != nil {
		return nil, err
	}

	// use upstream helper package to return an exporter that implements the required interface, and has timeout,
	// queueing and retry feature enabled
	prwexp, err := exporterhelper.NewMetricsExporter(
		cfg,
		params.Logger,
		prwe.PushMetrics,
		exporterhelper.WithTimeout(prwCfg.TimeoutSettings),
		exporterhelper.WithQueue(prwCfg.QueueSettings),
		exporterhelper.WithRetry(prwCfg.RetrySettings),
		exporterhelper.WithShutdown(prwe.Shutdown),
	)

	return prwexp, err
}

func createDefaultConfig() configmodels.Exporter {
	qs := exporterhelper.CreateDefaultQueueSettings()
	qs.Enabled = false

	ts := exporterhelper.CreateDefaultRetrySettings()
	ts.Enabled = false

	return &Config{
		Config: prw.Config{
			ExporterSettings: configmodels.ExporterSettings{
				TypeVal: typeStr,
				NameVal: typeStr,
			},
			Namespace:       "",
			ExternalLabels:  map[string]string{},
			TimeoutSettings: exporterhelper.CreateDefaultTimeoutSettings(),
			RetrySettings:   ts,
			QueueSettings:   qs,
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: "http://some.url:9411/api/prom/push",
				// We almost read 0 bytes, so no need to tune ReadBufferSize.
				ReadBufferSize:  0,
				WriteBufferSize: 512 * 1024,
				Timeout:         exporterhelper.CreateDefaultTimeoutSettings().Timeout,
				Headers:         map[string]string{},
			},
		},
		AuthSettings: AuthSettings{
			Region:  "",
			Service: "",
		},
	}
}

func validateAuthConfig(params AuthSettings) bool {
	return !(params.Region != "" && params.Service == "" || params.Region == "" && params.Service != "")
}

func applyAuth(params AuthSettings) bool {
	return params.Region != "" && params.Service != ""
}
