// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"errors"
	"go.uber.org/multierr"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// defaultNamespace value is used as ssoTracesExporter.Namespace when component.Config.Namespace is not set.
	defaultNamespace = "namespace"

	// defaultDataset value is used as ssoTracesExporter.Dataset when component.Config.Dataset is not set.
	defaultDataset = "default"
)

// Config defines configuration for OpenSearch exporter.
type Config struct {
	confighttp.HTTPClientSettings  `mapstructure:"http"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	Namespace                      string `mapstructure:"namespace"`
	Dataset                        string `mapstructure:"dataset"`
}

var (
	errConfigNoEndpoint = errors.New("endpoint must be specified")
	errDatasetNoValue   = errors.New("dataset must be specified")
	errNamespaceNoValue = errors.New("namespace must be specified")
)

// Validate validates the opensearch server configuration.
func (cfg *Config) Validate() error {
	var multiErr []error
	if len(cfg.Endpoint) == 0 {
		multiErr = append(multiErr, errConfigNoEndpoint)
	}
	if len(cfg.Dataset) == 0 {
		multiErr = append(multiErr, errDatasetNoValue)
	}
	if len(cfg.Namespace) == 0 {
		multiErr = append(multiErr, errNamespaceNoValue)
	}
	return multierr.Combine(multiErr...)
}
