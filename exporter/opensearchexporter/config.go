// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// defaultNamespace value is used as ssoTracesExporter.Namespace when component.Config.Namespace is not set.
	defaultNamespace = "namespace"

	// defaultDataset value is used as ssoTracesExporter.Dataset when component.Config.Dataset is not set.
	defaultDataset = "default"

	// defaultBulkAction value is used when component.Config.BulkAction is not set.
	defaultBulkAction = "create"
)

// Config defines configuration for OpenSearch exporter.
type Config struct {
	confighttp.HTTPClientSettings  `mapstructure:"http"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	MappingsSettings               `mapstructure:"mapping"`

	// The Observability indices would follow the recommended for immutable data stream ingestion pattern using
	// the data_stream concepts. See https://opensearch.org/docs/latest/dashboards/im-dashboards/datastream/
	// Index pattern will follow the next naming template ss4o_{type}-{dataset}-{namespace}
	Namespace string `mapstructure:"namespace"`
	Dataset   string `mapstructure:"dataset"`

	// BulkAction configures the action for ingesting data. Only `create` and `index` are allowed here.
	// If not specified, the default value `create` will be used.
	BulkAction string `mapstructure:"bulk_action"`
}

var (
	errConfigNoEndpoint  = errors.New("endpoint must be specified")
	errDatasetNoValue    = errors.New("dataset must be specified")
	errNamespaceNoValue  = errors.New("namespace must be specified")
	errBulkActionInvalid = errors.New("bulk_action can either be `create` or `index`")
)

type MappingsSettings struct {
	// Mode configures the field mappings.
	Mode string `mapstructure:"mode"`

	// Additional field mappings.
	Fields map[string]string `mapstructure:"fields"`

	// File to read additional fields mappings from.
	File string `mapstructure:"file"`

	// Field to store timestamp in.  If not set uses the default @timestamp
	TimestampField string `mapstructure:"timestamp_field"`

	// Whether to store timestamp in Epoch miliseconds
	UnixTimestamp bool `mapstructure:"unix_timestamp"`

	// Try to find and remove duplicate fields
	Dedup bool `mapstructure:"dedup"`

	Dedot bool `mapstructure:"dedot"`
}

type MappingMode int

// Enum values for MappingMode.
const (
	MappingNone MappingMode = iota
	MappingECS
	MappingFlattenAttributes
)

func (m MappingMode) String() string {
	switch m {
	case MappingNone:
		return ""
	case MappingECS:
		return "ecs"
	case MappingFlattenAttributes:
		return "flatten_attributes"
	default:
		return ""
	}
}

var mappingModes = func() map[string]MappingMode {
	table := map[string]MappingMode{}
	for _, m := range []MappingMode{
		MappingNone,
		MappingECS,
		MappingFlattenAttributes,
	} {
		table[strings.ToLower(m.String())] = m
	}

	// config aliases
	table["no"] = MappingNone
	table["none"] = MappingNone

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

	if len(cfg.BulkAction) == 0 {
		cfg.BulkAction = "create"
	}

	if cfg.BulkAction != "create" && cfg.BulkAction != "index" {
		return errBulkActionInvalid
	}

	if _, ok := mappingModes[cfg.MappingsSettings.Mode]; !ok {
		return fmt.Errorf("unknown mapping mode %v", cfg.MappingsSettings.Mode)
	}

	return errors.Join(multiErr...)
}
