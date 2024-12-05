// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

// metric name and its associated search as a key value pair
var searchDict = map[string]string{
	`SplunkLicenseIndexUsageSearch`:       `search=search earliest=-10m latest=now index=_internal source=*license_usage.log type="Usage"| fields idx, b| eval indexname = if(len(idx)=0 OR isnull(idx),"(UNKNOWN)",idx)| stats sum(b) as b by indexname| eval By=round(b, 9)| fields indexname, By`,
	`SplunkIndexerAvgRate`:                `search=search earliest=-10m latest=now index=_telemetry | stats count(index) | appendcols [| rest splunk_server_group=dmc_group_indexer splunk_server_group="dmc_group_indexer" /services/server/introspection/indexer | eval average_KBps = round(average_KBps, 0) | eval status = if((reason == ".") OR (reason == "") OR isnull(reason), status, status.": ".reason) | fields splunk_server, average_KBps, status] | eval host = splunk_server | stats avg(average_KBps) as "indexer_avg_kbps", values(status) as "status" by host | fields host, indexer_avg_kbps`,
	`SplunkSchedulerAvgExecLatencySearch`: `search=search earliest=-10m latest=now index=_internal host=* sourcetype=scheduler (status="completed" OR status="skipped" OR status="deferred" OR status="success") | eval window_time = if(isnull('window_time'), 0, 'window_time') | eval execution_latency = max(0.00, ('dispatch_time' - (scheduled_time %2B window_time))) | stats avg(execution_latency) AS avg_exec_latency by host | eval host = if(isnull(host), "(UNKNOWN)", host) | eval latency_avg_exec = round(avg_exec_latency, 2) | fields host, latency_avg_exec`,
	`SplunkSchedulerCompletionRatio`:      `search=search earliest=-10m latest=now index=_internal host=* sourcetype=scheduler (status="completed" OR status="skipped" OR status="deferred" OR status="success") | stats count(eval(status=="completed" OR status=="skipped" OR status="success")) AS total_exec, count(eval(status=="skipped")) AS skipped_exec by host | eval completion_ratio = round((1-(skipped_exec / total_exec)) * 100, 2) | fields host, completion_ratio`,
	`SplunkSchedulerAvgRunTime`:           `search=search earliest=-10m latest=now index=_internal host=* sourcetype=scheduler (status="completed" OR status="skipped" OR status="deferred" OR status="success") | eval runTime = avg(run_time) | stats avg(runTime) AS runTime by host | eval host = if(isnull(host), "(UNKNOWN)", host) | eval run_time_avg = round(runTime, 2) | fields host, run_time_avg`,
	`SplunkIndexerRawWriteSeconds`:        `search=search earliest=-10m latest=now index=_internal host=* source=*metrics.log sourcetype=splunkd group=pipeline name=indexerpipe processor=indexer | eval ingest_pipe = if(isnotnull(ingest_pipe), ingest_pipe, "none") | search ingest_pipe=* | stats sum(write_cpu_seconds) AS "raw_data_write_seconds" by host | fields host, raw_data_write_seconds`,
	`SplunkIndexerCpuSeconds`:             `search=search earliest=-10m latest=now index=_internal host=* source=*metrics.log sourcetype=splunkd group=pipeline name=indexerpipe processor=indexer | eval ingest_pipe = if(isnotnull(ingest_pipe), ingest_pipe, "none") | search ingest_pipe=* | stats sum(service_cpu_seconds) AS "service_cpu_seconds" by host | fields host, service_cpu_seconds`,
	`SplunkIoAvgIops`:                     `search=search earliest=-10m latest=now index=_introspection sourcetype=splunk_resource_usage component=IOStats host=* | eval mount_point = 'data.mount_point' | eval reads_ps = 'data.reads_ps' | eval writes_ps = 'data.writes_ps' | eval interval = 'data.interval' | eval total_io = reads_ps %2B writes_ps| eval op_count = (interval * total_io)| stats avg(op_count) as iops by host| eval iops = round(iops) | fields host, iops`,
	`SplunkPipelineQueues`:                `search=search earliest=-10m latest=now index=_telemetry | stats count(index) | appendcols [| rest splunk_server_group=dmc_group_indexer splunk_server_group="dmc_group_indexer" /services/server/introspection/queues | search title=parsingQueue* OR title=aggQueue* OR title=typingQueue* OR title=indexQueue* | eval fill_perc=round(current_size_bytes / max_size_bytes * 100,2) | fields splunk_server, title, fill_perc | rex field=title %22%28%3F%3Cqueue_name%3E%5E%5Cw%2B%29%28%3F%3A%5C.%28%3F%3Cpipeline_number%3E%5Cd%2B%29%29%3F%22 | eval fill_perc = if(isnotnull(pipeline_number), "pset".pipeline_number.": ".fill_perc, fill_perc) | chart values(fill_perc) over splunk_server by queue_name | eval pset_count = mvcount(parsingQueue)] | eval host = splunk_server | stats sum(pset_count) as "pipeline_sets", sum(parsingQueue) as "parse_queue_ratio", sum(aggQueue) as "agg_queue_ratio", sum(typingQueue) as "typing_queue_ratio", sum(indexQueue) as "index_queue_ratio" by host | fields host, pipeline_sets, parse_queue_ratio, agg_queue_ratio, typing_queue_ratio, index_queue_ratio`,
	`SplunkBucketsSearchableStatus`:       `search=search earliest=-10m latest=now index=_telemetry | stats count(index) | appendcols [| rest splunk_server_group=dmc_group_cluster_master splunk_server_group=* /services/cluster/master/peers | eval splunk_server = label | fields splunk_server, label, is_searchable, status, site, bucket_count, host_port_pair, last_heartbeat, replication_port, base_generation_id, title, bucket_count_by_index.* | eval is_searchable = if(is_searchable == 1 or is_searchable == "1", "Yes", "No")] | sort - last_heartbeat | search label="***" | search is_searchable="*" | search status="*" | search site="*" | eval host = splunk_server | stats values(is_searchable) as is_searchable, values(status) as status, avg(bucket_count) as bucket_count by host | fields host, is_searchable, status, bucket_count`,
	`SplunkIndexesData`:                   `search=search earliest=-10m latest=now index=_telemetry | stats count(index) | appendcols [| rest splunk_server_group=dmc_group_indexer splunk_server_group="*" /services/data/indexes] | join title splunk_server type=outer [ rest splunk_server_group=dmc_group_indexer splunk_server_group="*" /services/data/indexes-extended ] | eval elapsedTime = now() - strptime(minTime,"%25Y-%25m-%25dT%25H%3A%25M%3A%25S%25z") | eval dataAge = ceiling(elapsedTime / 86400) | eval indexSizeGB = if(currentDBSizeMB >= 1 AND totalEventCount >=1, currentDBSizeMB/1024, null()) | eval maxSizeGB = maxTotalDataSizeMB / 1024 | eval sizeUsagePerc = indexSizeGB / maxSizeGB * 100 | stats dc(splunk_server) AS splunk_server_count count(indexSizeGB) as "non_empty_instances" sum(indexSizeGB) AS total_size_gb avg(indexSizeGB) as average_size_gb avg(sizeUsagePerc) as average_usage_perc median(dataAge) as median_data_age max(dataAge) as oldest_data_age latest(bucket_dirs.home.warm_bucket_count) as warm_bucket_count latest(bucket_dirs.home.hot_bucket_count) as hot_bucket_count by title, datatype | eval warm_bucket_count = if(isnotnull(warm_bucket_count), warm_bucket_count, 0)| eval hot_bucket_count = if(isnotnull(hot_bucket_count), hot_bucket_count, 0)| eval bucket_count = (warm_bucket_count %2B hot_bucket_count)| eval total_size_gb = if(isnotnull(total_size_gb), round(total_size_gb, 2), 0) | eval average_size_gb = if(isnotnull(average_size_gb), round(average_size_gb, 2), 0) | eval average_usage_perc = if(isnotnull(average_usage_perc), round(average_usage_perc, 2), 0) | eval median_data_age = if(isNum(median_data_age), median_data_age, 0) | eval oldest_data_age = if(isNum(oldest_data_age), oldest_data_age, 0) | fields title splunk_server_count non_empty_instances total_size_gb average_size_gb average_usage_perc median_data_age bucket_count warm_bucket_count hot_bucket_count`,
	`SplunkIndexesBucketCounts`:           `search=search earliest=-10m latest=now index=_telemetry | stats count(index) | appendcols [| rest splunk_server_group=dmc_group_cluster_master splunk_server_group=* /services/cluster/master/indexes | fields title, is_searchable, replicated_copies_tracker*, searchable_copies_tracker*, num_buckets, index_size] | rename replicated_copies_tracker.*.* as rp**, searchable_copies_tracker.*.* as sb** | foreach rp0actual_copies_per_slot [ eval replicated_data_copies_ratio = ('rp0actual_copies_per_slot' / 'rp0expected_total_per_slot') ] | foreach sb0actual_copies_per_slot [ eval searchable_data_copies_ratio = ('sb0actual_copies_per_slot' / 'sb0expected_total_per_slot')] | eval is_searchable = if((is_searchable == 1) or (is_searchable == "1"), "Yes", "No") | eval index_size_gb = round(index_size / 1024 / 1024 / 1024, 2) | fields title, is_searchable, searchable_data_copies_ratio, replicated_data_copies_ratio, num_buckets, index_size_gb | search title="***" | search is_searchable="*" | stats latest(searchable_data_copies_ratio) as searchable_data_copies_ratio, latest(replicated_data_copies_ratio) as replicated_data_copies_ratio, latest(num_buckets) as num_buckets, latest(index_size_gb) as index_size_gb by title | fields title searchable_data_copies_ratio replicated_data_copies_ratio num_buckets index_size_gb`,
}

var apiDict = map[string]string{
	`SplunkIndexerThroughput`:   `/services/server/introspection/indexer?output_mode=json`,
	`SplunkDataIndexesExtended`: `/services/data/indexes-extended?output_mode=json&count=-1`,
	`SplunkIntrospectionQueues`: `/services/server/introspection/queues?output_mode=json&count=-1`,
	`SplunkKVStoreStatus`:       `/services/kvstore/status?output_mode=json`,
	`SplunkDispatchArtifacts`:   `/services/server/status/dispatch-artifacts?output_mode=json&count=-1`,
	`SplunkHealth`:              `/services/server/health/splunkd/details?output_mode=json`,
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

// '/services/kvstore/status'
const (
	// unknown/failed values
	KVStatusUnknown        = "unknown"
	KVRestoreStatusUnknown = "Unknown status"
	KVBackupStatusFailed   = "Failed"
)

type KVStoreStatus struct {
	Entries []KVEntry `json:"entry"`
}

type KVEntry struct {
	Content KVStatus `json:"content"`
}

type KVStatus struct {
	Current   KVStoreCurrent `json:"current"`
	KVService KVService      `json:"externalKVStore,omitempty"`
}

type KVService struct {
	Status string `json:"status"`
}

type KVStoreCurrent struct {
	Status              string `json:"status"`
	BackupRestoreStatus string `json:"backupRestoreStatus"`
	ReplicationStatus   string `json:"replicationStatus"`
	StorageEngine       string `json:"storageEngine"`
}

// '/services/server/status/dispatch-artifacts'
type DispatchArtifacts struct {
	Entries []DispatchArtifactEntry `json:"entry"`
}

type DispatchArtifactEntry struct {
	Content DispatchArtifactContent `json:"content"`
}

type DispatchArtifactContent struct {
	AdhocCount         string `json:"adhoc_count"`
	ScheduledCount     string `json:"scheduled_count"`
	SavedSearchesCount string `json:"ss_count"`
	CompletedCount     string `json:"completed_count"`
	IncompleteCount    string `json:"incomple_count"`
	InvalidCount       string `json:"invalid_count"`
	InfoCacheSize      string `json:"cached_job_status_info_csv_size_mb"`
	StatusCacheSize    string `json:"cached_job_status_status_csv_size_mb"`
	CacheTotalEntries  string `json:"cached_job_status_total_entries"`
}

// '/services/server/health/splunkd/details
type HealthArtifacts struct {
	Entries []HealthArtifactEntry `json:"entry"`
}

type HealthArtifactEntry struct {
	Content HealthDetails `json:"content"`
}

type HealthDetails struct {
	Health   string                   `json:"health"`
	Features map[string]HealthDetails `json:"features,omitempty"`
}
