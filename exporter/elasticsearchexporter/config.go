// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Elastic exporter.
type Config struct {
	exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	// Endpoints holds the Elasticsearch URLs the exporter should send events to.
	//
	// This setting is required if CloudID is not set and if the
	// ELASTICSEARCH_URL environment variable is not set.
	Endpoints []string `mapstructure:"endpoints"`

	// CloudID holds the cloud ID to identify the Elastic Cloud cluster to send events to.
	// https://www.elastic.co/guide/en/cloud/current/ec-cloud-id.html
	//
	// This setting is required if no URL is configured.
	CloudID string `mapstructure:"cloudid"`

	// NumWorkers configures the number of workers publishing bulk requests.
	NumWorkers int `mapstructure:"num_workers"`

	// Index configures the index, index alias, or data stream name events should be indexed in.
	//
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/indices.html
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/data-streams.html
	//
	// Deprecated: `index` is deprecated and replaced with `logs_index`.
	Index string `mapstructure:"index"`

	// This setting is required when logging pipelines used.
	LogsIndex string `mapstructure:"logs_index"`
	// fall back to pure LogsIndex, if 'elasticsearch.index.prefix' or 'elasticsearch.index.suffix' are not found in resource or attribute (prio: resource > attribute)
	LogsDynamicIndex DynamicIndexSetting `mapstructure:"logs_dynamic_index"`
	// This setting is required when traces pipelines used.
	TracesIndex string `mapstructure:"traces_index"`
	// fall back to pure TracesIndex, if 'elasticsearch.index.prefix' or 'elasticsearch.index.suffix' are not found in resource or attribute (prio: resource > attribute)
	TracesDynamicIndex DynamicIndexSetting `mapstructure:"traces_dynamic_index"`

	// Pipeline configures the ingest node pipeline name that should be used to process the
	// events.
	//
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/ingest.html
	Pipeline string `mapstructure:"pipeline"`

	HTTPClientSettings `mapstructure:",squash"`
	Discovery          DiscoverySettings `mapstructure:"discover"`
	Retry              RetrySettings     `mapstructure:"retry"`
	Flush              FlushSettings     `mapstructure:"flush"`
	Mapping            MappingsSettings  `mapstructure:"mapping"`
}

type DynamicIndexSetting struct {
	Enabled bool `mapstructure:"enabled"`
}

type HTTPClientSettings struct {
	Authentication AuthenticationSettings `mapstructure:",squash"`

	// ReadBufferSize for HTTP client. See http.Transport.ReadBufferSize.
	ReadBufferSize int `mapstructure:"read_buffer_size"`

	// WriteBufferSize for HTTP client. See http.Transport.WriteBufferSize.
	WriteBufferSize int `mapstructure:"write_buffer_size"`

	// Timeout configures the HTTP request timeout.
	Timeout time.Duration `mapstructure:"timeout"`

	// Headers allows users to configure optional HTTP headers that
	// will be send with each HTTP request.
	Headers map[string]string `mapstructure:"headers,omitempty"`

	configtls.TLSClientSetting `mapstructure:"tls,omitempty"`
}

// AuthenticationSettings defines user authentication related settings.
type AuthenticationSettings struct {
	// User is used to configure HTTP Basic Authentication.
	User string `mapstructure:"user"`

	// Password is used to configure HTTP Basic Authentication.
	Password configopaque.String `mapstructure:"password"`

	// APIKey is used to configure ApiKey based Authentication.
	//
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/security-api-create-api-key.html
	APIKey configopaque.String `mapstructure:"api_key"`
}

// DiscoverySettings defines Elasticsearch node discovery related settings.
// The exporter will check Elasticsearch regularly for available nodes
// and updates the list of hosts if discovery is enabled. Newly discovered
// nodes will automatically be used for load balancing.
//
// DiscoverySettings should not be enabled when operating Elasticsearch behind a proxy
// or load balancer.
//
// https://www.elastic.co/blog/elasticsearch-sniffing-best-practices-what-when-why-how
type DiscoverySettings struct {
	// OnStart, if set, instructs the exporter to look for available Elasticsearch
	// nodes the first time the exporter connects to the cluster.
	OnStart bool `mapstructure:"on_start"`

	// Interval instructs the exporter to renew the list of Elasticsearch URLs
	// with the given interval. URLs will not be updated if Interval is <=0.
	Interval time.Duration `mapstructure:"interval"`
}

// FlushSettings  defines settings for configuring the write buffer flushing
// policy in the Elasticsearch exporter. The exporter sends a bulk request with
// all events already serialized into the send-buffer.
type FlushSettings struct {
	// Bytes sets the send buffer flushing limit.
	Bytes int `mapstructure:"bytes"`

	// Interval configures the max age of a document in the send buffer.
	Interval time.Duration `mapstructure:"interval"`
}

// RetrySettings defines settings for the HTTP request retries in the Elasticsearch exporter.
// Failed sends are retried with exponential backoff.
type RetrySettings struct {
	// Enabled allows users to disable retry without having to comment out all settings.
	Enabled bool `mapstructure:"enabled"`

	// MaxRequests configures how often an HTTP request is retried before it is assumed to be failed.
	MaxRequests int `mapstructure:"max_requests"`

	// InitialInterval configures the initial waiting time if a request failed.
	InitialInterval time.Duration `mapstructure:"initial_interval"`

	// MaxInterval configures the max waiting time if consecutive requests failed.
	MaxInterval time.Duration `mapstructure:"max_interval"`
}

type MappingsSettings struct {
	// Mode configures the field mappings.
	Mode string `mapstructure:"mode"`

	// Additional field mappings.
	Fields map[string]string `mapstructure:"fields"`

	// File to read additional fields mappings from.
	File string `mapstructure:"file"`

	// Try to find and remove duplicate fields
	Dedup bool `mapstructure:"dedup"`

	Dedot bool `mapstructure:"dedot"`
}

type MappingMode int

// Enum values for MappingMode.
const (
	MappingNone MappingMode = iota
	MappingECS
	MappingJaeger
)

var (
	errConfigNoEndpoint    = errors.New("endpoints or cloudid must be specified")
	errConfigEmptyEndpoint = errors.New("endpoints must not include empty entries")
)

func (m MappingMode) String() string {
	switch m {
	case MappingNone:
		return ""
	case MappingECS:
		return "ecs"
	case MappingJaeger:
		return "jaeger"
	default:
		return ""
	}
}

var mappingModes = func() map[string]MappingMode {
	table := map[string]MappingMode{}
	for _, m := range []MappingMode{
		MappingNone,
		MappingECS,
		MappingJaeger,
	} {
		table[strings.ToLower(m.String())] = m
	}

	// config aliases
	table["no"] = MappingNone
	table["none"] = MappingNone

	return table
}()

const defaultElasticsearchEnvName = "ELASTICSEARCH_URL"

// Validate validates the elasticsearch server configuration.
func (cfg *Config) Validate() error {
	if len(cfg.Endpoints) == 0 && cfg.CloudID == "" {
		if os.Getenv(defaultElasticsearchEnvName) == "" {
			return errConfigNoEndpoint
		}
	}

	for _, endpoint := range cfg.Endpoints {
		if endpoint == "" {
			return errConfigEmptyEndpoint
		}
	}

	if _, ok := mappingModes[cfg.Mapping.Mode]; !ok {
		return fmt.Errorf("unknown mapping mode %v", cfg.Mapping.Mode)
	}

	return nil
}
