// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicegraphconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"

import (
	"time"
)

// Config defines the configuration options for servicegraphprocessor.
type Config struct {
	// MetricsExporter is the name of the metrics exporter to use to ship metrics.
	//
	// Deprecated: The exporter is defined as part of the pipeline and this option is currently noop.
	MetricsExporter string `mapstructure:"metrics_exporter"`

	// LatencyHistogramBuckets is the list of durations representing latency histogram buckets.
	// See defaultLatencyHistogramBucketsMs in processor.go for the default value.
	LatencyHistogramBuckets []time.Duration `mapstructure:"latency_histogram_buckets"`

	// Dimensions defines the list of additional dimensions on top of the provided:
	// - client
	// - server
	// - failed
	// - connection_type
	// The dimensions will be fetched from the span's attributes. Examples of some conventionally used attributes:
	// https://github.com/open-telemetry/opentelemetry-collector/blob/main/model/semconv/opentelemetry.go.
	Dimensions []string `mapstructure:"dimensions"`

	// Store contains the config for the in-memory store used to find requests between services by pairing spans.
	Store StoreConfig `mapstructure:"store"`

	// CacheLoop is the time to cleans the cache periodically.
	CacheLoop time.Duration `mapstructure:"cache_loop"`

	// CacheLoop is the time to expire old entries from the store periodically.
	StoreExpirationLoop time.Duration `mapstructure:"store_expiration_loop"`

	// VirtualNodePeerAttributes the list of attributes need to match, the higher the front, the higher the priority.
	VirtualNodePeerAttributes []string `mapstructure:"virtual_node_peer_attributes"`

	// VirtualNodeExtraLabel enables the `virtual_node` label to be added to the spans.
	VirtualNodeExtraLabel bool `mapstructure:"virtual_node_extra_label"`

	// MetricsFlushInterval is the interval at which metrics are flushed to the exporter.
	// If set to 0, metrics are flushed on every received batch of traces.
	// Default is 60s if unset.
	MetricsFlushInterval *time.Duration `mapstructure:"metrics_flush_interval"`

	// DatabaseNameAttribute is the attribute name used to identify the database name from span attributes.
	// The default value is db.name.
	// Deprecated: [v0.124.0] Use database_name_attributes instead.
	DatabaseNameAttribute string `mapstructure:"database_name_attribute"`

	// DatabaseNameAttributes is the attribute name list of attributes need to match used to identify the database name from span attributes, the higher the front, the higher the priority.
	// The default value is {"db.name"}.
	DatabaseNameAttributes []string `mapstructure:"database_name_attributes"`
}

type StoreConfig struct {
	// MaxItems is the maximum number of items to keep in the store.
	MaxItems int `mapstructure:"max_items"`
	// TTL is the time to live for items in the store.
	TTL time.Duration `mapstructure:"ttl"`
}
