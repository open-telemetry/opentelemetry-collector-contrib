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

// Package awsprometheusremotewriteexporter provides a Prometheus Remote Write Exporter with AWS Sigv4 authentication
package awsprometheusremotewriteexporter

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"

	prw "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"
)

const typeStr = "awsprometheusremotewrite" // The value of "type" key in configuration.

type awsFactory struct {
	component.ExporterFactory
}

// NewFactory returns a factory of the AWS Prometheus Remote Write exporter that can be registered to the Collector.
func NewFactory() component.ExporterFactory {
	return &awsFactory{ExporterFactory: prw.NewFactory()}
}

func (af *awsFactory) Type() config.Type {
	return typeStr
}

func (af *awsFactory) CreateMetricsExporter(ctx context.Context, params component.ExporterCreateSettings,
	cfg config.Exporter) (component.MetricsExporter, error) {
	return af.ExporterFactory.CreateMetricsExporter(ctx, params, &cfg.(*Config).Config)
}

func (af *awsFactory) CreateDefaultConfig() config.Exporter {
	cfg := &Config{
		Config: *af.ExporterFactory.CreateDefaultConfig().(*prw.Config),
		AuthConfig: AuthConfig{
			Region:  "",
			Service: defaultAMPSigV4Service,
			RoleArn: "",
		},
	}

	cfg.ExporterSettings = config.NewExporterSettings(config.NewID(typeStr))
	cfg.HTTPClientSettings.CustomRoundTripper = func(next http.RoundTripper) (http.RoundTripper, error) {
		extras := []string{runtime.Version(), runtime.GOOS, runtime.GOARCH}
		if v := os.Getenv("AWS_EXECUTION_ENV"); v != "" {
			extras = append(extras, v)
		}
		runtimeInfo := fmt.Sprintf("%s/%s (%s)", aws.SDKName, aws.SDKVersion, strings.Join(extras, "; "))
		return newSigningRoundTripper(cfg, next, runtimeInfo)
	}

	return cfg
}
