// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlesecopsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlesecopsexporter"

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
	"google.golang.org/grpc/encoding/gzip"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlesecopsexporter/internal/expr"
)

const (
	chronicleAPI      = "chronicle"
	backstoryAPI      = "backstory"
	apiVersionV1Alpha = "v1alpha"
	apiVersionV1Beta  = "v1beta"
)

// Config defines configuration for the Google SecOps Exporter.
type Config struct {
	// API is the API that will be used to send logs to Google SecOps
	// Either "chronicle" or "backstory".
	API string `mapstructure:"api"`

	// ServiceEndpoint is the base service endpoint used to construct the API endpoints.
	ServiceEndpoint string `mapstructure:"service_endpoint"`

	// CustomerID is the customer ID that will be used to send logs to Google SecOps.
	CustomerID string `mapstructure:"customer_id"`

	// APIVersion is the version of the Chronicle API to use. Defaults to "v1alpha".
	// Only used for the Chronicle API.
	APIVersion string `mapstructure:"api_version"`

	// Region is the GCP region of the Google SecOps instance to send logs to.
	// Only used for the Chronicle API.
	Region string `mapstructure:"region"`

	// ProjectNumber is the GCP project number of the Google SecOps instance to send logs to.
	// Only used for the Chronicle API.
	ProjectNumber string `mapstructure:"project_number"`

	// Namespace is the namespace that will be used to send logs to Google SecOps.
	Namespace string `mapstructure:"namespace"`

	// Auth configures authentication via a collector auth extension such as
	// googleclientauth. If unset, the exporter relies on Application Default
	// Credentials for authentication.
	Auth configoptional.Optional[configauth.Config] `mapstructure:"auth"`

	// DefaultLogType is the type of log that will be sent to Google SecOps if not overridden by `attributes["log_type"]`, `attributes["chronicle_log_type"]`, or `attributes["google_secops.log.type"]`.
	DefaultLogType string `mapstructure:"default_log_type"`

	// ValidateLogTypes is a flag that determines whether or not to validate the log types using an API call.
	ValidateLogTypes bool `mapstructure:"validate_log_types"`

	// RawLogField is the field name that will be used to send raw logs to Google SecOps.
	RawLogField string `mapstructure:"raw_log_field"`

	// Compression is the compression type that will be used to send logs to Google SecOps.
	Compression string `mapstructure:"compression"`

	// IngestionLabels are the labels that will be attached to logs when sent to Google SecOps.
	IngestionLabels map[string]string `mapstructure:"ingestion_labels"`

	// CollectAgentMetrics is a flag that determines whether or not to collect agent metrics.
	CollectAgentMetrics bool `mapstructure:"collect_agent_metrics"`

	// MetricsInterval is the interval at which to collect and send agent metrics.
	MetricsInterval time.Duration `mapstructure:"metrics_interval"`

	// BatchRequestSizeLimit is the maximum batch request size, in bytes, that can be sent to Google SecOps
	// This field is defaulted to 4000000 as that is the default limit
	// Setting this option to a value above the backend limit may result in rejected log batch requests
	BatchRequestSizeLimit int `mapstructure:"batch_request_size_limit"`

	// LogErroredPayloads is a flag that determines whether or not to log errored payloads.
	LogErroredPayloads bool `mapstructure:"log_errored_payloads"`

	// CollectorID is the collector ID of the ingestion method. Leave empty to use the default collector ID (recommended).
	CollectorID string `mapstructure:"collector_id"`

	TimeoutConfig    exporterhelper.TimeoutConfig                             `mapstructure:",squash"`
	QueueBatchConfig configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
	BackOffConfig    configretry.BackOffConfig                                `mapstructure:"retry_on_failure"`
}

// Validate checks if the configuration is valid.
func (cfg *Config) Validate() error {
	if cfg.ServiceEndpoint == "" {
		return errors.New("service_endpoint is required")
	}

	_, err := url.Parse(cfg.ServiceEndpoint)
	if err != nil {
		return fmt.Errorf("invalid service_endpoint: %w", err)
	}

	if cfg.CustomerID == "" {
		return errors.New("customer_id is required")
	}

	if cfg.Compression != "" && cfg.Compression != gzip.Name {
		return fmt.Errorf("invalid compression type: %s", cfg.Compression)
	}

	if cfg.BatchRequestSizeLimit <= 0 {
		return errors.New("batch_request_size_limit must be greater than 0")
	}

	switch cfg.API {
	case chronicleAPI:
		if cfg.Region == "" {
			return errors.New("region is required for the Chronicle API")
		}
		if cfg.ProjectNumber == "" {
			return errors.New("project_number is required for the Chronicle API")
		}
		if cfg.APIVersion != "" {
			if cfg.APIVersion != apiVersionV1Alpha && cfg.APIVersion != apiVersionV1Beta {
				return fmt.Errorf("invalid api_version: %s", cfg.APIVersion)
			}
		}
	case backstoryAPI:
	case "":
		return errors.New("api is required")
	default:
		return fmt.Errorf("invalid API: %s", cfg.API)
	}

	if cfg.RawLogField != "" {
		_, err := expr.NewOTTLLogRecordExpression(cfg.RawLogField, component.TelemetrySettings{
			Logger: zap.NewNop(),
		})
		if err != nil {
			return fmt.Errorf("invalid raw_log_field: %w", err)
		}
	}

	if cfg.CollectorID != "" {
		if uuid.Validate(cfg.CollectorID) != nil {
			return errors.New("invalid collector_id")
		}
	}

	return nil
}
