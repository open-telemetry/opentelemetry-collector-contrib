// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"errors"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// defaultNamespace value is used as ssoTracesExporter.Namespace when component.Config.Namespace is not set.
	defaultNamespace = "namespace"

	// defaultDataset value is used as ssoTracesExporter.Dataset when component.Config.Dataset is not set.
	defaultDataset = "default"

	// defaultBulkAction value is used when component.Config.BulkAction is not set.
	defaultBulkAction = "create"

	// defaultMappingMode value is used when component.Config.MappingSettings.Mode is not set.
	defaultMappingMode = "ss4o"
)

// Config defines configuration for OpenSearch exporter.
type Config struct {
	confighttp.ClientConfig   `mapstructure:"http"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	TimeoutSettings           exporterhelper.TimeoutConfig `mapstructure:",squash"`
	MappingsSettings          `mapstructure:"mapping"`
	QueueConfig               exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`

	// The Observability indices would follow the recommended for immutable data stream ingestion pattern using
	// the data_stream concepts. See https://opensearch.org/docs/latest/dashboards/im-dashboards/datastream/
	// Index pattern will follow the next naming template ss4o_{type}-{dataset}-{namespace}
	Dataset   string `mapstructure:"dataset"`
	Namespace string `mapstructure:"namespace"`

	// LogsIndex configures the index, index alias, or data stream name logs should be indexed in.
	// https://opensearch.org/docs/latest/im-plugin/index/
	// https://opensearch.org/docs/latest/dashboards/im-dashboards/datastream/
	LogsIndex string `mapstructure:"logs_index"`

	// BulkAction configures the action for ingesting data. Only `create` and `index` are allowed here.
	// If not specified, the default value `create` will be used.
	BulkAction string `mapstructure:"bulk_action"`
}

var (
	errConfigNoEndpoint   = errors.New("endpoint must be specified")
	errDatasetNoValue     = errors.New("dataset must be specified")
	errNamespaceNoValue   = errors.New("namespace must be specified")
	errBulkActionInvalid  = errors.New("bulk_action can either be `create` or `index`")
	errMappingModeInvalid = errors.New("mapping.mode is invalid")
)

type MappingsSettings struct {
	// Mode configures the field mappings.
	// Supported modes are the following:
	//
	//   ss4o: exports logs in the Simple Schema for Observability standard.
	//   This mode is enabled by default.
	//   See: https://opensearch.org/docs/latest/observing-your-data/ss4o/
	//
	//   ecs: maps fields defined in the OpenTelemetry Semantic Conventions
	//   to the Elastic Common Schema.
	//   See: https://www.elastic.co/guide/en/ecs/current/index.html
	//
	//   flatten_attributes: uses the ECS mapping but flattens all resource and
	//   log attributes in the record to the top-level.
	Mode string `mapstructure:"mode"`

	// Additional field mappings.
	Fields map[string]string `mapstructure:"fields"`

	// File to read additional fields mappings from.
	File string `mapstructure:"file"`

	// Field to store timestamp in.  If not set uses the default @timestamp
	TimestampField string `mapstructure:"timestamp_field"`

	// Whether to store timestamp in Epoch milliseconds
	UnixTimestamp bool `mapstructure:"unix_timestamp"`

	// Try to find and remove duplicate fields
	Dedup bool `mapstructure:"dedup"`

	Dedot bool `mapstructure:"dedot"`
}

type MappingMode int

// Enum values for MappingMode.
const (
	MappingSS4O MappingMode = iota
	MappingECS
	MappingFlattenAttributes
)

func (m MappingMode) String() string {
	switch m {
	case MappingSS4O:
		return "ss4o"
	case MappingECS:
		return "ecs"
	case MappingFlattenAttributes:
		return "flatten_attributes"
	default:
		return "ss4o"
	}
}

var mappingModes = func() map[string]MappingMode {
	table := map[string]MappingMode{}
	for _, m := range []MappingMode{
		MappingECS,
		MappingSS4O,
		MappingFlattenAttributes,
	} {
		table[strings.ToLower(m.String())] = m
	}

	return table
}()

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

	if cfg.BulkAction != "create" && cfg.BulkAction != "index" {
		return errBulkActionInvalid
	}

	if _, ok := mappingModes[cfg.MappingsSettings.Mode]; !ok {
		multiErr = append(multiErr, errMappingModeInvalid)
	}

	return errors.Join(multiErr...)
}
