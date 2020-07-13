package query

import (
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
)

type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`

	confighttp.HTTPClientSettings `mapstructure:",squash"`

	// Index that's being queried. If none is provided, given query will be
	// applied across all indexes. To apply the search query to multiple indices,
	// provide a comma separated list of indices
	Index string `mapstructure:"index"`

	// Takes in an Elasticsearch request body search request. See
	// [here] (https://www.elastic.co/guide/en/elasticsearch/reference/current/search-request-body.html)
	// for details.
	ElasticsearchRequest string `mapstructure:"elasticsearch_request"`

	// The duration between ElasticSearch query execution.
	CollectionInterval time.Duration `mapstructure:"collection_interval"`
}

var DefaultConfig = Config{
	CollectionInterval: 10 * time.Second,
}
