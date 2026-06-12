// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"errors"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
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
	QueueConfig               configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	// The Observability indices would follow the recommended for immutable data stream ingestion pattern using
	// the data_stream concepts. See https://opensearch.org/docs/latest/dashboards/im-dashboards/datastream/
	// Index pattern will follow the next naming template ss4o_{type}-{dataset}-{namespace}
	Dataset   string `mapstructure:"dataset"`
	Namespace string `mapstructure:"namespace"`

	// LogsIndex configures the index, index alias, or data stream name logs should be indexed in.
	// https://opensearch.org/docs/latest/im-plugin/index/
	// https://opensearch.org/docs/latest/dashboards/im-dashboards/datastream/
	LogsIndex           string `mapstructure:"logs_index"`
	LogsIndexFallback   string `mapstructure:"logs_index_fallback"`
	LogsIndexTimeFormat string `mapstructure:"logs_index_time_format"`

	// TracesIndex configures the index, index alias, or data stream name traces should be indexed in.
	// https://opensearch.org/docs/latest/im-plugin/index/
	// https://opensearch.org/docs/latest/dashboards/im-dashboards/datastream/
	TracesIndex           string `mapstructure:"traces_index"`
	TracesIndexFallback   string `mapstructure:"traces_index_fallback"`
	TracesIndexTimeFormat string `mapstructure:"traces_index_time_format"`

	// BulkAction configures the action for ingesting data. Only `create` and `index` are allowed here.
	// If not specified, the default value `create` will be used.
	BulkAction string `mapstructure:"bulk_action"`

	// Pipeline is the optional ID of an ingest pipeline to apply when indexing documents.
	// https://opensearch.org/docs/latest/ingest-pipelines/
	Pipeline string `mapstructure:"pipeline"`
}

var (
	errConfigNoEndpoint               = errors.New("endpoint must be specified")
	errDatasetNoValue                 = errors.New("dataset must be specified")
	errNamespaceNoValue               = errors.New("namespace must be specified")
	errBulkActionInvalid              = errors.New("bulk_action can either be `create` or `index`")
	errMappingModeInvalid             = errors.New("mapping.mode is invalid")
	errLogsIndexTimeFormatInvalid     = errors.New("logs_index_time_format contains unsupported or invalid tokens")
	errTracesIndexTimeFormatInvalid   = errors.New("traces_index_time_format contains unsupported or invalid tokens")
	errOTelV1DatasetNamespaceUnused   = errors.New(`dataset and namespace are not used by mapping.mode "otel-v1"; remove them or pick a different mode`)
	errManageIndexTemplateInvalidMode = errors.New("mapping.manage_index_template is only supported with mapping.mode \"otel-v1\"")
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
	//
	//   bodymap: supports only logs and uses the "body" of a log record as the exact content
	//   of the OpenSearch document, without any transformation.
	//   This mapping mode is intended for use cases where the client wishes to have complete control over the
	//   OpenSearch document structure.
	//
	//   otel-v1: exports logs and traces using the OTel v1 schema published by the
	//   OpenSearch Data Prepper project (https://github.com/opensearch-project/data-prepper).
	//   Documents are compatible with OpenSearch Observability dashboards that consume
	//   Data Prepper indices. See the upstream index templates for the canonical field mappings:
	//     https://github.com/opensearch-project/data-prepper/blob/main/data-prepper-plugins/opensearch/src/main/resources/index-template/otel-v1-apm-span-index-standard-template.json
	//     https://github.com/opensearch-project/data-prepper/blob/main/data-prepper-plugins/opensearch/src/main/resources/index-template/logs-otel-v1-index-standard-template.json
	Mode string `mapstructure:"mode"`

	// ManageIndexTemplate controls whether the exporter creates index templates on startup.
	// Only supported when Mode is "otel-v1". Validation will reject this option with other modes.
	ManageIndexTemplate bool `mapstructure:"manage_index_template"`

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
	MappingBodyMap
	MappingOTelV1
)

func (m MappingMode) String() string {
	switch m {
	case MappingSS4O:
		return "ss4o"
	case MappingECS:
		return "ecs"
	case MappingFlattenAttributes:
		return "flatten_attributes"
	case MappingBodyMap:
		return "bodymap"
	case MappingOTelV1:
		return "otel-v1"
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
		MappingBodyMap,
		MappingOTelV1,
	} {
		table[strings.ToLower(m.String())] = m
	}

	return table
}()

// Validate validates the opensearch server configuration.
func (cfg *Config) Validate() error {
	var multiErr []error
	if cfg.Endpoint == "" {
		multiErr = append(multiErr, errConfigNoEndpoint)
	}

	if cfg.Mode == MappingOTelV1.String() {
		// otel-v1 emits Data Prepper-style index names (otel-v1-apm-span,
		// otel-v1-logs); dataset/namespace would be silently dropped, so reject
		// any user-supplied (i.e. non-default) values up front instead of
		// surprising the user. Defaults from newDefaultConfig are tolerated so
		// that switching mode does not require the user to also clear two
		// otherwise-irrelevant fields.
		if (cfg.Dataset != "" && cfg.Dataset != defaultDataset) ||
			(cfg.Namespace != "" && cfg.Namespace != defaultNamespace) {
			multiErr = append(multiErr, errOTelV1DatasetNamespaceUnused)
		}
	} else {
		if cfg.Dataset == "" {
			multiErr = append(multiErr, errDatasetNoValue)
		}
		if cfg.Namespace == "" {
			multiErr = append(multiErr, errNamespaceNoValue)
		}
	}

	// Validate LogsIndexTimeFormat if set
	if cfg.LogsIndexTimeFormat != "" {
		if err := validateTimeFormat(cfg.LogsIndexTimeFormat); err != nil {
			multiErr = append(multiErr, errLogsIndexTimeFormatInvalid)
		}
	}

	// Validate TracesIndexTimeFormat if set
	if cfg.TracesIndexTimeFormat != "" {
		if err := validateTimeFormat(cfg.TracesIndexTimeFormat); err != nil {
			multiErr = append(multiErr, errTracesIndexTimeFormatInvalid)
		}
	}

	if cfg.BulkAction != "create" && cfg.BulkAction != "index" {
		return errBulkActionInvalid
	}

	if _, ok := mappingModes[cfg.Mode]; !ok {
		multiErr = append(multiErr, errMappingModeInvalid)
	}

	if cfg.ManageIndexTemplate && cfg.Mode != MappingOTelV1.String() {
		multiErr = append(multiErr, errManageIndexTemplateInvalidMode)
	}

	return errors.Join(multiErr...)
}

// validateTimeFormat validates a time format string contains only valid tokens and separators
func validateTimeFormat(format string) error {
	validTokens := []string{"yyyy", "yy", "MM", "dd", "HH", "mm", "ss"}
	remaining := format
	for _, token := range validTokens {
		remaining = strings.ReplaceAll(remaining, token, "")
	}
	// After removing all valid tokens, only allowed separators should remain
	allowed := "-._+"
	for _, r := range remaining {
		if !strings.ContainsRune(allowed, r) {
			return errors.New("invalid time format")
		}
	}
	return nil
}
