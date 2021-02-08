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

package f5cloudexporter

import (
	"context"
	"net/http"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	otlphttp "go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
)

const typeStr = "f5cloud" // The value of "type" key in configuration.

type TokenSourceGetter func(config *Config) (oauth2.TokenSource, error)

type f5cloudFactory struct {
	component.ExporterFactory
	getTokenSource TokenSourceGetter
}

// NewFactory returns a factory of the F5 Cloud exporter that can be registered to the Collector.
func NewFactory() component.ExporterFactory {
	return NewFactoryWithTokenSourceGetter(getTokenSourceFromConfig)
}

func NewFactoryWithTokenSourceGetter(tsg TokenSourceGetter) component.ExporterFactory {
	return &f5cloudFactory{ExporterFactory: otlphttp.NewFactory(), getTokenSource: tsg}
}

func (f *f5cloudFactory) Type() configmodels.Type {
	return typeStr
}

func (f *f5cloudFactory) CreateMetricsExporter(
	ctx context.Context,
	params component.ExporterCreateParams,
	config configmodels.Exporter) (component.MetricsExporter, error) {

	cfg := config.(*Config)

	if err := cfg.sanitize(); err != nil {
		return nil, err
	}

	fillUserAgent(cfg, params.ApplicationStartInfo.Version)

	return f.ExporterFactory.CreateMetricsExporter(ctx, params, &cfg.Config)
}

func (f *f5cloudFactory) CreateTracesExporter(
	ctx context.Context,
	params component.ExporterCreateParams,
	config configmodels.Exporter) (component.TracesExporter, error) {

	cfg := config.(*Config)

	if err := cfg.sanitize(); err != nil {
		return nil, err
	}

	fillUserAgent(cfg, params.ApplicationStartInfo.Version)

	return f.ExporterFactory.CreateTracesExporter(ctx, params, &cfg.Config)
}

func (f *f5cloudFactory) CreateLogsExporter(
	ctx context.Context,
	params component.ExporterCreateParams,
	config configmodels.Exporter) (component.LogsExporter, error) {

	cfg := config.(*Config)

	if err := cfg.sanitize(); err != nil {
		return nil, err
	}

	fillUserAgent(cfg, params.ApplicationStartInfo.Version)

	return f.ExporterFactory.CreateLogsExporter(ctx, params, &cfg.Config)
}

func (f *f5cloudFactory) CreateDefaultConfig() configmodels.Exporter {
	cfg := &Config{
		Config: *f.ExporterFactory.CreateDefaultConfig().(*otlphttp.Config),
		AuthConfig: AuthConfig{
			CredentialFile: "",
			Audience:       "",
		},
	}

	cfg.TypeVal = typeStr
	cfg.NameVal = typeStr

	cfg.Headers["User-Agent"] = "opentelemetry-collector-contrib {{version}}"

	cfg.HTTPClientSettings.CustomRoundTripper = func(next http.RoundTripper) (http.RoundTripper, error) {
		ts, err := f.getTokenSource(cfg)
		if err != nil {
			return nil, err
		}

		return newF5CloudAuthRoundTripper(ts, cfg.Source, next)
	}

	return cfg
}

// getTokenSourceFromConfig gets a TokenSource based on the configuration.
func getTokenSourceFromConfig(config *Config) (oauth2.TokenSource, error) {
	ts, err := idtoken.NewTokenSource(context.Background(), config.AuthConfig.Audience, idtoken.WithCredentialsFile(config.AuthConfig.CredentialFile))
	if err != nil {
		return nil, err
	}

	return ts, nil
}

func fillUserAgent(cfg *Config, version string) {
	cfg.Headers["User-Agent"] = strings.ReplaceAll(cfg.Headers["User-Agent"], "{{version}}", version)
}
