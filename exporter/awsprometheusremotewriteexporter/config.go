package awsprometheusremotewriteexporter

import (
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Remote Write exporter.
type Config struct {
	// squash ensures fields are correctly decoded in embedded struct.
	configmodels.ExporterSettings  `mapstructure:",squash"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	// prefix attached to each exported metric name
	// See: https://prometheus.io/docs/practices/naming/#metric-names
	Namespace string `mapstructure:"namespace"`

	// AWS Sig V4 configuration options
	AuthSettings AuthSettings `mapstructure:"aws_auth"`

	HTTPClientSettings confighttp.HTTPClientSettings `mapstructure:",squash"`
}

// AuthSettings defines AWS authentication configurations for SigningRoundTripper
type AuthSettings struct {
	// region string for AWS Sig V4
	Region string `mapstructure:"region"`
	// service string for AWS Sig V4
	Service string `mapstructure:"service"`
}
