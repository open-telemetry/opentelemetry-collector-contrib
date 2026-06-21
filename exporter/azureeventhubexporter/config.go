// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubexporter"

import (
	"errors"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

var (
	errMissingConnection      = errors.New("missing connection: set either 'connection' or 'auth' + 'event_hub.namespace' + 'event_hub.name'")
	errLogsPartitionExclusive = errors.New("partition_logs_by_resource_attributes and partition_logs_by_trace_id cannot both be enabled")
)

// Config defines the configuration for the Azure Event Hub exporter.
type Config struct {
	// Connection is the Azure Event Hub connection string.
	// Mutually exclusive with Auth; one of the two must be set.
	Connection string `mapstructure:"connection"`

	// EventHub holds the Event Hub name and namespace used when Auth is configured.
	EventHub EventHubConfig `mapstructure:"event_hub"`

	// Auth is the component ID of an auth extension that implements azcore.TokenCredential
	// (e.g. the azureauthextension). When set, Connection is ignored.
	Auth *component.ID `mapstructure:"auth"`

	// PartitionTracesByID sets the partition key of outgoing trace messages to the trace ID,
	// ensuring all spans belonging to the same trace land on the same Event Hub partition.
	PartitionTracesByID bool `mapstructure:"partition_traces_by_id"`

	// PartitionMetricsByResourceAttributes splits the outgoing metrics batch by resource and
	// sets the partition key to a hash of each resource's attributes, so metrics from the
	// same resource are consistently routed to the same partition.
	PartitionMetricsByResourceAttributes bool `mapstructure:"partition_metrics_by_resource_attributes"`

	// PartitionLogsByResourceAttributes splits the outgoing logs batch by resource and sets
	// the partition key to a hash of each resource's attributes.
	// Mutually exclusive with PartitionLogsByTraceID.
	PartitionLogsByResourceAttributes bool `mapstructure:"partition_logs_by_resource_attributes"`

	// PartitionLogsByTraceID sets the partition key of outgoing log messages to the trace ID
	// found on each log record, co-locating logs with their associated traces.
	// Mutually exclusive with PartitionLogsByResourceAttributes.
	PartitionLogsByTraceID bool `mapstructure:"partition_logs_by_trace_id"`

	// TimeoutSettings controls per-request timeouts.
	TimeoutSettings exporterhelper.TimeoutConfig `mapstructure:"timeout"`

	// QueueSettings configures the sending queue / batcher.
	QueueSettings configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	// BackOffConfig configures retry-on-failure behaviour.
	BackOffConfig configretry.BackOffConfig `mapstructure:"retry_on_failure"`
}

// EventHubConfig identifies the target Event Hub when Auth-based authentication is used.
type EventHubConfig struct {
	// Name is the name of the Event Hub (required when using Auth).
	Name string `mapstructure:"name"`
	// Namespace is the fully qualified namespace, e.g. "mynamespace.servicebus.windows.net"
	// (required when using Auth).
	Namespace string `mapstructure:"namespace"`
}

func (c *Config) Validate() error {
	if c.Auth != nil {
		if c.EventHub.Name == "" {
			return errors.New("event_hub.name is required when using auth")
		}
		if c.EventHub.Namespace == "" {
			return errors.New("event_hub.namespace is required when using auth")
		}
	} else {
		if c.Connection == "" {
			return errMissingConnection
		}
		if _, err := azeventhubs.ParseConnectionString(c.Connection); err != nil {
			return err
		}
	}
	if c.PartitionLogsByResourceAttributes && c.PartitionLogsByTraceID {
		return errLogsPartitionExclusive
	}
	return nil
}
