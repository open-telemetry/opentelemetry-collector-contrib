package stats

//IndexStatsOutput contains all the index level stats of
// the following types and will be used to retrieve node
// level metrics
type IndexStatsOutput struct {
	Shards        Shards                `json:"_shards"`
	AllIndexStats IndexStats            `json:"_all"`
	Indices       map[string]IndexStats `json:"indices"`
}

// Shards of the index
type Shards struct {
	Total      *int64 `json:"total"`
	Successful *int64 `json:"successful"`
	Failed     *int64 `json:"failed"`
}

// IndexStats holds stats from Primary shards or
// aggregate across all the shards
type IndexStats struct {
	Primaries IndexStatsGroups `json:"primaries"`
	Total     IndexStatsGroups `json:"total"`
}

// IndexStatsGroups is a list of all groups of stats available
// across all supported versions of Elasticsearch
type IndexStatsGroups struct {
	Docs         Docs         `json:"docs"`
	Store        Store        `json:"store"`
	Indexing     Indexing     `json:"indexing"`
	Get          Get          `json:"get"`
	Search       Search       `json:"search"`
	Merges       Merges       `json:"merges"`
	Refresh      Refresh      `json:"refresh"`
	Flush        Flush        `json:"flush"`
	Warmer       Warmer       `json:"warmer"`
	QueryCache   QueryCache   `json:"query_cache"`
	FilterCache  FilterCache  `json:"filter_cache"`
	Fielddata    Fielddata    `json:"fielddata"`
	Completion   Completion   `json:"completion"`
	Segments     Segments     `json:"segments"`
	Translog     Translog     `json:"translog"`
	RequestCache RequestCache `json:"request_cache"`
	Recovery     Recovery     `json:"recovery"`
	IDCache      IDCache      `json:"id_cache"`
	Suggest      Suggest      `json:"suggest"`
	Percolate    Percolate    `json:"percolate"`
}

// Docs stats available across all supported versions of Elasticsearch
type Docs struct {
	Count   *int64 `json:"count"`
	Deleted *int64 `json:"deleted"`
}

// Store stats available across all supported versions of Elasticsearch
type Store struct {
	SizeInBytes          *int64 `json:"size_in_bytes"`
	ThrottleTimeInMillis *int64 `json:"throttle_time_in_millis"`
}

// Indexing stats available across all supportedversions of Elasticsearch
type Indexing struct {
	IndexTotal           *int64 `json:"index_total"`
	IndexTimeInMillis    *int64 `json:"index_time_in_millis"`
	IndexCurrent         *int64 `json:"index_current"`
	IndexFailed          *int64 `json:"index_failed"`
	DeleteTotal          *int64 `json:"delete_total"`
	DeleteTimeInMillis   *int64 `json:"delete_time_in_millis"`
	DeleteCurrent        *int64 `json:"delete_current"`
	NoopUpdateTotal      *int64 `json:"noop_update_total"`
	ThrottleTimeInMillis *int64 `json:"throttle_time_in_millis"`
}

// Get stats available across all supportedversions of Elasticsearch
type Get struct {
	Total               *int64 `json:"total"`
	TimeInMillis        *int64 `json:"time_in_millis"`
	ExistsTotal         *int64 `json:"exists_total"`
	ExistsTimeInMillis  *int64 `json:"exists_time_in_millis"`
	MissingTotal        *int64 `json:"missing_total"`
	MissingTimeInMillis *int64 `json:"missing_time_in_millis"`
	Current             *int64 `json:"current"`
}

// Search stats available across all supported versions of Elasticsearch
type Search struct {
	OpenContexts        *int64 `json:"open_contexts"`
	QueryTotal          *int64 `json:"query_total"`
	QueryTimeInMillis   *int64 `json:"query_time_in_millis"`
	QueryCurrent        *int64 `json:"query_current"`
	FetchTotal          *int64 `json:"fetch_total"`
	FetchTimeInMillis   *int64 `json:"fetch_time_in_millis"`
	FetchCurrent        *int64 `json:"fetch_current"`
	ScrollTotal         *int64 `json:"scroll_total"`
	ScrollTimeInMillis  *int64 `json:"scroll_time_in_millis"`
	ScrollCurrent       *int64 `json:"scroll_current"`
	SuggestTotal        *int64 `json:"suggest_total"`
	SuggestTimeInMillis *int64 `json:"suggest_time_in_millis"`
	SuggestCurrent      *int64 `json:"suggest_current"`
}

// Merges stats available across all supportedversions of Elasticsearch
type Merges struct {
	Current                    *int64 `json:"current"`
	CurrentDocs                *int64 `json:"current_docs"`
	CurrentSizeInBytes         *int64 `json:"current_size_in_bytes"`
	Total                      *int64 `json:"total"`
	TotalTimeInMillis          *int64 `json:"total_time_in_millis"`
	TotalDocs                  *int64 `json:"total_docs"`
	TotalSizeInBytes           *int64 `json:"total_size_in_bytes"`
	TotalStoppedTimeInMillis   *int64 `json:"total_stopped_time_in_millis"`
	TotalThrottledTimeInMillis *int64 `json:"total_throttled_time_in_millis"`
	TotalAutoThrottleInBytes   *int64 `json:"total_auto_throttle_in_bytes"`
}

// Refresh stats available across all supported versions of Elasticsearch
type Refresh struct {
	Total             *int64 `json:"total"`
	TotalTimeInMillis *int64 `json:"total_time_in_millis"`
	Listeners         *int64 `json:"listeners"`
}

// Flush stats available across all supported versions of Elasticsearch
type Flush struct {
	Total             *int64 `json:"total"`
	Periodic          *int64 `json:"periodic"`
	TotalTimeInMillis *int64 `json:"total_time_in_millis"`
}

// Warmer stats available across all supported versions of Elasticsearch
type Warmer struct {
	Current           *int64 `json:"current"`
	Total             *int64 `json:"total"`
	TotalTimeInMillis *int64 `json:"total_time_in_millis"`
}

// QueryCache stats available across all supported versions
// of Elasticsearch. Known as FilterCache until ES 2.0.
type QueryCache struct {
	MemorySizeInBytes *int64 `json:"memory_size_in_bytes"`
	TotalCount        *int64 `json:"total_count"`
	HitCount          *int64 `json:"hit_count"`
	MissCount         *int64 `json:"miss_count"`
	CacheSize         *int64 `json:"cache_size"`
	CacheCount        *int64 `json:"cache_count"`
	Evictions         *int64 `json:"evictions"`
}

// FilterCache stats available across all supported versions
// of Elasticsearch. Known as QuertyCache after ES 2.0.
type FilterCache struct {
	MemorySizeInBytes *int64 `json:"memory_size_in_bytes"`
	Evictions         *int64 `json:"evictions"`
}

// Fielddata stats available across all supported versions of Elasticsearch
type Fielddata struct {
	MemorySizeInBytes *int64 `json:"memory_size_in_bytes"`
	Evictions         *int64 `json:"evictions"`
}

// Completion stats available across all supported versions of Elasticsearch
type Completion struct {
	SizeInBytes *int64 `json:"size_in_bytes"`
}

// Segments stats available across all supported versions of Elasticsearch
type Segments struct {
	Count                       *int64 `json:"count"`
	MemoryInBytes               *int64 `json:"memory_in_bytes"`
	TermsMemoryInBytes          *int64 `json:"terms_memory_in_bytes"`
	StoredFieldsMemoryInBytes   *int64 `json:"stored_fields_memory_in_bytes"`
	TermVectorsMemoryInBytes    *int64 `json:"term_vectors_memory_in_bytes"`
	NormsMemoryInBytes          *int64 `json:"norms_memory_in_bytes"`
	PointsMemoryInBytes         *int64 `json:"points_memory_in_bytes"`
	DocValuesMemoryInBytes      *int64 `json:"doc_values_memory_in_bytes"`
	IndexWriterMemoryInBytes    *int64 `json:"index_writer_memory_in_bytes"`
	IndexWriterMaxMemoryInBytes *int64 `json:"index_writer_max_memory_in_bytes"`
	VersionMapMemoryInBytes     *int64 `json:"version_map_memory_in_bytes"`
	FixedBitSetMemoryInBytes    *int64 `json:"fixed_bit_set_memory_in_bytes"`
}

// Translog stats available across all supported versions of Elasticsearch
type Translog struct {
	Operations              *int64 `json:"operations"`
	SizeInBytes             *int64 `json:"size_in_bytes"`
	UncommittedOperations   *int64 `json:"uncommitted_operations"`
	UncommittedSizeInBytes  *int64 `json:"uncommitted_size_in_bytes"`
	EarliestLastModifiedAge *int64 `json:"earliest_last_modified_age"`
}

// RequestCache stats available across all supported versions of Elasticsearch
type RequestCache struct {
	MemorySizeInBytes *int64 `json:"memory_size_in_bytes"`
	Evictions         *int64 `json:"evictions"`
	HitCount          *int64 `json:"hit_count"`
	MissCount         *int64 `json:"miss_count"`
}

// Recovery stats available across all supported ersions of Elasticsearch
type Recovery struct {
	CurrentAsSource      *int64 `json:"current_as_source"`
	CurrentAsTarget      *int64 `json:"current_as_target"`
	ThrottleTimeInMillis *int64 `json:"throttle_time_in_millis"`
}

// IDCache stats available across all supported versions of
// Elasticsearch. Deprecated since ES 2.0.
type IDCache struct {
	MemorySizeInBytes *int64 `json:"memory_size_in_bytes"`
}

// Suggest stats available across all supported versions of
// Elasticsearch. Deprecated since ES 2.0.
type Suggest struct {
	Total        *int64 `json:"total"`
	TimeInMillis *int64 `json:"time_in_millis"`
	Current      *int64 `json:"current"`
}

// Percolate stats available across all supported versions
// of Elasticsearch. Deprecated since ES 5.0.
type Percolate struct {
	Total        *int64 `json:"total"`
	TimeInMillis *int64 `json:"time_in_millis"`
	Current      *int64 `json:"current"`
	Queries      *int64 `json:"queries"`
}
