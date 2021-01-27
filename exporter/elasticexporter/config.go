// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package elasticexporter

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtls"
)

// Config defines configuration for Elastic exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"`
	configtls.TLSClientSetting    `mapstructure:",squash"`

	APMServerConfig     `mapstructure:",squash"`
	ElasticsearchConfig `mapstructure:",squash"`
}

// APMServerConfig defines settings for connecting to the APM Server.
type APMServerConfig struct {
	// APMServerURLs holds the APM Server URL.
	//
	// This is required.
	APMServerURL string `mapstructure:"apm_server_url"`

	// APIKey holds an optional API Key for authorization.
	//
	// https://www.elastic.co/guide/en/apm/server/7.7/api-key-settings.html
	APIKey string `mapstructure:"api_key"`

	// SecretToken holds the optional secret token for authorization.
	//
	// https://www.elastic.co/guide/en/apm/server/7.7/secret-token.html
	SecretToken string `mapstructure:"secret_token"`
}

// ElasticsearchConfig defines settings for the Elasticsearch Exporter.
type ElasticsearchConfig struct {
	// URLs holds the Elasticsearch URLs the exporter should send events to.
	//
	// This setting is required if CloudID is not set and if the
	// ELASTICSEARCH_URL environment variable is not set.
	URLs []string `mapstructure:"elasticsearch_urls"`

	// Workers configures the number of workers publishing bulk requests.
	Workers int `mapstructure:"workers"`

	// CloudID holds the cloud ID to identify the Elastic Cloud cluster to send events to.
	// https://www.elastic.co/guide/en/cloud/current/ec-cloud-id.html
	//
	// This setting is required if no URL is configured.
	CloudID string `mapstructure:"cloudid"`

	// ReadBufferSize for HTTP client. See http.Transport.ReadBufferSize.
	ReadBufferSize int `mapstructure:"read_buffer_size"`

	// WriteBufferSize for HTTP client. See http.Transport.WriteBufferSize.
	WriteBufferSize int `mapstructure:"write_buffer_size"`

	// Timeout configures the HTTP request timeout.
	Timeout time.Duration `mapstructure:"timeout"`

	// Headers allows users to configure optional HTTP headers that
	// will be send with each HTTP request.
	Headers map[string]string `mapstructure:"headers,omitempty"`

	// Index configures the index, index alias, or data stream name events should be indexed in.
	//
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/indices.html
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/data-streams.html
	//
	// This setting is required.
	Index string `mapstructure:"index"`

	// Pipeline configures the ingest node pipeline name that should be used to process the
	// events.
	//
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/ingest.html
	Pipeline string `mapstructure:"pipeline"`

	Authentication ElasticsearchAuthentication `mapstructure:",squash"`
	Discovery      Discovery                   `mapstructure:"discover"`
	Retry          RetryConfig                 `mapstructure:"retry"`
	Flush          FlushConfig                 `mapstructure:"flush"`
	Mapping        MappingsConfig              `mapstructure:"mapping"`
}

// ElasticsearchAuthentication defines user authentication related settings.
type ElasticsearchAuthentication struct {
	// User is used to configure HTTP Basic Authentication.
	User string `mapstructure:"user"`

	// Password is used to configure HTTP Basic Authentication.
	Password string `mapstructure:"password"`

	// APIKey is used to configure ApiKey based Authentication.
	//
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/security-api-create-api-key.html
	APIKey string `mapstructure:"elasticsearch_api_key"`
}

// Discovery defines Elasticsearch node discovery related settings.
// The exporter will check Elasticsearch regularily for available nodes
// and updates the list of hosts if discovery is enabled. Newly discovered
// nodes will automatically be used for load balancing.
//
// Discovery should not be enabled when operating Elasticsearch behind a proxy
// or load balancer.
//
// https://www.elastic.co/blog/elasticsearch-sniffing-best-practices-what-when-why-how
type Discovery struct {
	// OnStart, if set, instructs the exporter to look for available Elasticsearch
	// nodes the first time the exporter connects to the cluster.
	OnStart bool `mapstructure:"on_start"`

	// Interval instructs the exporter to renew the list of Elasticsearch URLs
	// with the given interval. URLs will not be updated if Interval is <=0.
	Interval time.Duration `mapstructure:"interval"`
}

// FlushConfig  defines settings for configuring the write buffer flushing
// policy in the Elasticsearch exporter. The exporter sends a bulk request with
// all events already serialized into the send-buffer.
type FlushConfig struct {
	// Bytes sets the send buffer flushing limit.
	Bytes int `mapstructure:"bytes"`

	// Interval configures the max age of a document in the send buffer.
	Interval time.Duration `mapstructure:"interval"`
}

// RetryConfig defines settings for the HTTP request retries in the Elasticsearch exporter.
// Failed sends are retried with exponential backoff.
type RetryConfig struct {
	// Enabled allows users to disable retry without having to comment out all settings.
	Enabled bool `mapstructure:"enabled"`

	// Max configures how often an HTTP request is retried before it is assumed to be failed.
	Max int `mapstructure:"max"`

	// InitialInterval configures the initial waiting time if a request failed.
	InitialInterval time.Duration `mapstructure:"initial_interval"`

	// MaxInterval configures the max waiting time if consecutive requests failed.
	MaxInterval time.Duration `mapstructure:"max_interval"`
}

type MappingsConfig struct {
	// Mode configures the field mappings.
	Mode string `mapstructure:"mode"`

	// File to read additional fields mappings from.
	File string `mapstructure:"file"`
}

type MappingMode int

const (
	MappingNone MappingMode = iota
	MappingECS
)

func (m MappingMode) String() string {
	switch m {
	case MappingNone:
		return ""
	case MappingECS:
		return "ecs"
	default:
		return ""
	}
}

var mappingModes = func() map[string]MappingMode {
	table := map[string]MappingMode{}
	for _, m := range []MappingMode{
		MappingNone,
		MappingECS,
	} {
		table[strings.ToLower(m.String())] = m
	}

	// config aliases
	table["no"] = MappingNone
	table["none"] = MappingNone

	return table
}()

// Validate validates the apm server configuration.
func (cfg APMServerConfig) Validate() error {
	if cfg.APMServerURL == "" {
		return errors.New("APMServerURL must be specified")
	}
	return nil
}

// Validate validates the elasticsearch server configuration.
func (cfg ElasticsearchConfig) Validate() error {
	if len(cfg.URLs) == 0 && cfg.CloudID == "" {
		return errors.New("Elasticsearch URL or CloudID must be specified")
	}

	for _, url := range cfg.URLs {
		if url == "" {
			return errors.New("Elasticsearch URL must not be empty")
		}
	}

	if cfg.Index == "" {
		return errors.New("Elasticsearch Index must be specified")
	}

	if _, ok := mappingModes[cfg.Mapping.Mode]; !ok {
		return fmt.Errorf("unknown mapping mode %v", cfg.Mapping.Mode)
	}

	return nil
}
