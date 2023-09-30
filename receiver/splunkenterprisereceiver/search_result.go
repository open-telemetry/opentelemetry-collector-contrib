// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

// metric name and its associated search as a key value pair
var searchDict = map[string]string{
	`SplunkLicenseIndexUsageSearch`: `search=search index=_internal source=*license_usage.log type="Usage"| fields idx, b| eval indexname = if(len(idx)=0 OR isnull(idx),"(UNKNOWN)",idx)| stats sum(b) as b by indexname| eval By=round(b, 9)| fields indexname, By`,
}

var apiDict = map[string]string{
	`SplunkIndexerThroughput`:   `/services/server/introspection/indexer?output_mode=json`,
	`SplunkDataIndexesExtended`: `/services/data/indexes-extended?output_mode=json&count=-1`,
	`SplunkIntrospectionQueues`: `/services/server/introspection/queues?output_mode=json&count=-1`,
}

type searchResponse struct {
	search string
	Jobid  *string `xml:"sid"`
	Return int
	Fields []*field `xml:"result>field"`
}

type field struct {
	FieldName string `xml:"k,attr"`
	Value     string `xml:"value>text"`
}

// '/services/server/introspection/indexer'
type indexThroughput struct {
	Entries []idxTEntry `json:"entry"`
}

type idxTEntry struct {
	Content idxTContent `json:"content"`
}

type idxTContent struct {
	Status string  `json:"status"`
	AvgKb  float64 `json:"average_KBps"`
}

// '/services/data/indexes-extended'
type IndexesExtended struct {
	Entries []IdxEEntry `json:"entry"`
}

type IdxEEntry struct {
	Name    string      `json:"name"`
	Content IdxEContent `json:"content"`
}

type IdxEContent struct {
	TotalBucketCount string         `json:"total_bucket_count"`
	TotalEventCount  int            `json:"totalEventCount"`
	TotalSize        string         `json:"total_size"`
	TotalRawSize     string         `json:"total_raw_size"`
	BucketDirs       IdxEBucketDirs `json:"bucket_dirs"`
}

type IdxEBucketDirs struct {
	Cold   IdxEBucketDirsDetails `json:"cold"`
	Home   IdxEBucketDirsDetails `json:"home"`
	Thawed IdxEBucketDirsDetails `json:"thawed"`
}

type IdxEBucketDirsDetails struct {
	Capacity        string `json:"capacity"`
	EventCount      string `json:"event_count"`
	EventMaxTime    string `json:"event_max_time"`
	EventMinTime    string `json:"event_min_time"`
	HotBucketCount  string `json:"hot_bucket_count"`
	WarmBucketCount string `json:"warm_bucket_count"`
	WarmBucketSize  string `json:"warm_bucket_size"`
}

// '/services/server/introspection/queues'
type IntrospectionQueues struct {
	Entries []IntrQEntry `json:"entry"`
}

type IntrQEntry struct {
	Name    string      `json:"name"`
	Content IdxQContent `json:"content"`
}

type IdxQContent struct {
	CurrentSize      int `json:"current_size"`
	CurrentSizeBytes int `json:"current_size_bytes"`
	LargestSize      int `json:"largest_size"`
	MaxSizeBytes     int `json:"max_size_bytes"`
}
