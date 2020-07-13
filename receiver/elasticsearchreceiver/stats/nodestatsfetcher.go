package stats

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	esUtils "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/utils"
)

// Groups of Node stats being that the monitor collects
const (
	TransportStatsGroup  = "transport"
	HTTPStatsGroup       = "http"
	JVMStatsGroup        = "jvm"
	ThreadpoolStatsGroup = "thread_pool"
	ProcessStatsGroup    = "process"
)

// GetNodeStatsMetrics fetches datapoints for ES Node stats
func GetNodeStatsMetrics(nodeStatsOutput *NodeStatsOutput, defaultDims map[string]string, selectedThreadPools map[string]bool, enhancedStatsForIndexGroups map[string]bool, nodeStatsGroupEnhancedOption map[string]bool) []*metricspb.Metric {
	var out []*metricspb.Metric
	for _, nodeStats := range nodeStatsOutput.NodeStats {
		out = append(out, getNodeStatsMetricsHelper(nodeStats, defaultDims, selectedThreadPools, enhancedStatsForIndexGroups, nodeStatsGroupEnhancedOption)...)
	}
	return out
}

func getNodeStatsMetricsHelper(nodeStats NodeStats, defaultDims map[string]string, selectedThreadPools map[string]bool, enhancedStatsForIndexGroups map[string]bool, nodeStatsGroupEnhancedOption map[string]bool) []*metricspb.Metric {
	var ms []*metricspb.Metric

	ms = append(ms, nodeStats.JVM.getJVMStats(nodeStatsGroupEnhancedOption[JVMStatsGroup], defaultDims)...)
	ms = append(ms, nodeStats.Process.getProcessStats(nodeStatsGroupEnhancedOption[ProcessStatsGroup], defaultDims)...)
	ms = append(ms, nodeStats.Transport.getTransportStats(nodeStatsGroupEnhancedOption[TransportStatsGroup], defaultDims)...)
	ms = append(ms, nodeStats.HTTP.getHTTPStats(nodeStatsGroupEnhancedOption[HTTPStatsGroup], defaultDims)...)
	ms = append(ms, fetchThreadPoolStats(nodeStatsGroupEnhancedOption[ThreadpoolStatsGroup], nodeStats.ThreadPool, defaultDims, selectedThreadPools)...)
	ms = append(ms, nodeStats.Indices.getIndexGroupStats(enhancedStatsForIndexGroups, defaultDims)...)

	return ms
}

func (jvm *JVM) getJVMStats(enhanced bool, dims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge("elasticsearch.jvm.threads.count", dims, jvm.JvmThreadsStats.Count),
			esUtils.PrepareGauge("elasticsearch.jvm.threads.peak", dims, jvm.JvmThreadsStats.PeakCount),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.heap-used-percent", dims, jvm.JvmMemStats.HeapUsedPercent),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.heap-max", dims, jvm.JvmMemStats.HeapMaxInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.non-heap-committed", dims, jvm.JvmMemStats.NonHeapCommittedInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.non-heap-used", dims, jvm.JvmMemStats.NonHeapUsedInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.pools.young.max_in_bytes", dims, jvm.JvmMemStats.Pools.Young.MaxInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.pools.young.used_in_bytes", dims, jvm.JvmMemStats.Pools.Young.UsedInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.pools.young.peak_used_in_bytes", dims, jvm.JvmMemStats.Pools.Young.PeakUsedInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.pools.young.peak_max_in_bytes", dims, jvm.JvmMemStats.Pools.Young.PeakMaxInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.pools.old.max_in_bytes", dims, jvm.JvmMemStats.Pools.Old.MaxInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.pools.old.used_in_bytes", dims, jvm.JvmMemStats.Pools.Old.UsedInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.pools.old.peak_used_in_bytes", dims, jvm.JvmMemStats.Pools.Old.PeakUsedInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.pools.old.peak_max_in_bytes", dims, jvm.JvmMemStats.Pools.Old.PeakMaxInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.pools.survivor.max_in_bytes", dims, jvm.JvmMemStats.Pools.Survivor.MaxInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.pools.survivor.used_in_bytes", dims, jvm.JvmMemStats.Pools.Survivor.UsedInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.pools.survivor.peak_used_in_bytes", dims, jvm.JvmMemStats.Pools.Survivor.PeakUsedInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.pools.survivor.peak_max_in_bytes", dims, jvm.JvmMemStats.Pools.Survivor.PeakMaxInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.buffer_pools.mapped.count", dims, jvm.BufferPools.Mapped.Count),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.buffer_pools.mapped.used_in_bytes", dims, jvm.BufferPools.Mapped.UsedInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.buffer_pools.mapped.total_capacity_in_bytes", dims, jvm.BufferPools.Mapped.TotalCapacityInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.buffer_pools.direct.count", dims, jvm.BufferPools.Direct.Count),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.buffer_pools.direct.used_in_bytes", dims, jvm.BufferPools.Direct.UsedInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.mem.buffer_pools.direct.total_capacity_in_bytes", dims, jvm.BufferPools.Direct.TotalCapacityInBytes),
			esUtils.PrepareGauge("elasticsearch.jvm.classes.current-loaded-count", dims, jvm.Classes.CurrentLoadedCount),

			esUtils.PrepareCumulative("elasticsearch.jvm.gc.count", dims, jvm.JvmGcStats.Collectors.Young.CollectionCount),
			esUtils.PrepareCumulative("elasticsearch.jvm.gc.old-count", dims, jvm.JvmGcStats.Collectors.Old.CollectionCount),
			esUtils.PrepareCumulative("elasticsearch.jvm.gc.old-time", dims, jvm.JvmGcStats.Collectors.Old.CollectionTimeInMillis),
			esUtils.PrepareCumulative("elasticsearch.jvm.classes.total-loaded-count", dims, jvm.Classes.TotalLoadedCount),
			esUtils.PrepareCumulative("elasticsearch.jvm.classes.total-unloaded-count", dims, jvm.Classes.TotalUnloadedCount),
		}...)
	}

	out = append(out, []*metricspb.Metric{
		esUtils.PrepareGauge("elasticsearch.jvm.mem.heap-used", dims, jvm.JvmMemStats.HeapUsedInBytes),
		esUtils.PrepareGauge("elasticsearch.jvm.mem.heap-committed", dims, jvm.JvmMemStats.HeapCommittedInBytes),
		esUtils.PrepareCumulative("elasticsearch.jvm.uptime", dims, jvm.UptimeInMillis),
		esUtils.PrepareCumulative("elasticsearch.jvm.gc.time", dims, jvm.JvmGcStats.Collectors.Young.CollectionTimeInMillis),
	}...)

	return out
}

func (processStats *Process) getProcessStats(enhanced bool, dims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge("elasticsearch.process.max_file_descriptors", dims, processStats.MaxFileDescriptors),
			esUtils.PrepareGauge("elasticsearch.process.cpu.percent", dims, processStats.CPU.Percent),
			esUtils.PrepareCumulative("elasticsearch.process.cpu.time", dims, processStats.CPU.TotalInMillis),
			esUtils.PrepareCumulative("elasticsearch.process.mem.total-virtual-size", dims, processStats.Mem.TotalVirtualInBytes),
		}...)
	}

	out = append(out, []*metricspb.Metric{
		esUtils.PrepareGauge("elasticsearch.process.open_file_descriptors", dims, processStats.OpenFileDescriptors),
	}...)

	return out
}

func fetchThreadPoolStats(enhanced bool, threadPools map[string]ThreadPoolStats, defaultDims map[string]string, selectedThreadPools map[string]bool) []*metricspb.Metric {
	var out []*metricspb.Metric
	for threadPool, stats := range threadPools {
		if !selectedThreadPools[threadPool] {
			continue
		}
		out = append(out, threadPoolMetrics(enhanced, threadPool, stats, defaultDims)...)
	}
	return out
}

func threadPoolMetrics(enhanced bool, threadPool string, threadPoolStats ThreadPoolStats, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric
	threadPoolDimension := map[string]string{}
	threadPoolDimension["thread_pool"] = threadPool

	dims := esUtils.MergeStringMaps(defaultDims, threadPoolDimension)

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge("elasticsearch.thread_pool.threads", dims, threadPoolStats.Threads),
			esUtils.PrepareGauge("elasticsearch.thread_pool.queue", dims, threadPoolStats.Queue),
			esUtils.PrepareGauge("elasticsearch.thread_pool.active", dims, threadPoolStats.Active),
			esUtils.PrepareGauge("elasticsearch.thread_pool.largest", dims, threadPoolStats.Largest),
			esUtils.PrepareCumulative("elasticsearch.thread_pool.completed", dims, threadPoolStats.Completed),
		}...)
	}

	out = append(out, []*metricspb.Metric{
		esUtils.PrepareCumulative("elasticsearch.thread_pool.rejected", dims, threadPoolStats.Rejected),
	}...)
	return out
}

func (transport *Transport) getTransportStats(enhanced bool, dims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge("elasticsearch.transport.server_open", dims, transport.ServerOpen),
			esUtils.PrepareCumulative("elasticsearch.transport.rx.count", dims, transport.RxCount),
			esUtils.PrepareCumulative("elasticsearch.transport.rx.size", dims, transport.RxSizeInBytes),
			esUtils.PrepareCumulative("elasticsearch.transport.tx.count", dims, transport.TxCount),
			esUtils.PrepareCumulative("elasticsearch.transport.tx.size", dims, transport.TxSizeInBytes),
		}...)
	}
	return out
}

func (http *HTTP) getHTTPStats(enhanced bool, dims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge("elasticsearch.http.current_open", dims, http.CurrentOpen),
			esUtils.PrepareCumulative("elasticsearch.http.total_open", dims, http.TotalOpened),
		}...)
	}

	return out
}

func (indexStatsGroups *IndexStatsGroups) getIndexGroupStats(enhancedStatsForIndexGroups map[string]bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	out = append(out, indexStatsGroups.Docs.getDocsStats(enhancedStatsForIndexGroups[DocsStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Store.getStoreStats(enhancedStatsForIndexGroups[StoreStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Indexing.getIndexingStats(enhancedStatsForIndexGroups[IndexingStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Get.getGetStats(enhancedStatsForIndexGroups[GetStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Search.getSearchStats(enhancedStatsForIndexGroups[SearchStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Merges.getMergesStats(enhancedStatsForIndexGroups[MergesStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Refresh.getRefreshStats(enhancedStatsForIndexGroups[RefreshStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Flush.getFlushStats(enhancedStatsForIndexGroups[FlushStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Warmer.getWarmerStats(enhancedStatsForIndexGroups[WarmerStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.QueryCache.getQueryCacheStats(enhancedStatsForIndexGroups[QueryCacheStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.FilterCache.getFilterCacheStats(enhancedStatsForIndexGroups[FilterCacheStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Fielddata.getFielddataStats(enhancedStatsForIndexGroups[FieldDataStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Completion.getCompletionStats(enhancedStatsForIndexGroups[CompletionStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Segments.getSegmentsStats(enhancedStatsForIndexGroups[SegmentsStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Translog.getTranslogStats(enhancedStatsForIndexGroups[TranslogStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.RequestCache.getRequestCacheStats(enhancedStatsForIndexGroups[RequestCacheStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Recovery.getRecoveryStats(enhancedStatsForIndexGroups[RecoveryStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.IDCache.getIDCacheStats(enhancedStatsForIndexGroups[IDCacheStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Suggest.getSuggestStats(enhancedStatsForIndexGroups[SuggestStatsGroup], defaultDims)...)
	out = append(out, indexStatsGroups.Percolate.getPercolateStats(enhancedStatsForIndexGroups[PercolateStatsGroup], defaultDims)...)

	return out
}

func (docs *Docs) getDocsStats(_ bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	out = append(out, []*metricspb.Metric{
		esUtils.PrepareGauge(elasticsearchIndicesDocsCount, defaultDims, docs.Count),
		esUtils.PrepareGauge(elasticsearchIndicesDocsDeleted, defaultDims, docs.Deleted),
	}...)

	return out
}

func (store *Store) getStoreStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareCumulative(elasticsearchIndicesStoreThrottleTime, defaultDims, store.ThrottleTimeInMillis),
		}...)
	}

	out = append(out, []*metricspb.Metric{
		esUtils.PrepareGauge(elasticsearchIndicesStoreSize, defaultDims, store.SizeInBytes),
	}...)

	return out
}

func (indexing Indexing) getIndexingStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesIndexingIndexCurrent, defaultDims, indexing.IndexCurrent),
			esUtils.PrepareGauge(elasticsearchIndicesIndexingIndexFailed, defaultDims, indexing.IndexFailed),
			esUtils.PrepareGauge(elasticsearchIndicesIndexingDeleteCurrent, defaultDims, indexing.DeleteCurrent),
			esUtils.PrepareCumulative(elasticsearchIndicesIndexingDeleteTotal, defaultDims, indexing.DeleteTotal),
			esUtils.PrepareCumulative(elasticsearchIndicesIndexingDeleteTime, defaultDims, indexing.DeleteTimeInMillis),
			esUtils.PrepareCumulative(elasticsearchIndicesIndexingNoopUpdateTotal, defaultDims, indexing.NoopUpdateTotal),
			esUtils.PrepareCumulative(elasticsearchIndicesIndexingThrottleTime, defaultDims, indexing.ThrottleTimeInMillis),
		}...)
	}

	out = append(out, []*metricspb.Metric{
		esUtils.PrepareCumulative(elasticsearchIndicesIndexingIndexTotal, defaultDims, indexing.IndexTotal),
		esUtils.PrepareCumulative(elasticsearchIndicesIndexingIndexTime, defaultDims, indexing.IndexTimeInMillis),
	}...)

	return out
}

func (get *Get) getGetStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesGetCurrent, defaultDims, get.Current),
			esUtils.PrepareCumulative(elasticsearchIndicesGetTime, defaultDims, get.TimeInMillis),
			esUtils.PrepareCumulative(elasticsearchIndicesGetExistsTotal, defaultDims, get.ExistsTotal),
			esUtils.PrepareCumulative(elasticsearchIndicesGetExistsTime, defaultDims, get.ExistsTimeInMillis),
			esUtils.PrepareCumulative(elasticsearchIndicesGetMissingTotal, defaultDims, get.MissingTotal),
			esUtils.PrepareCumulative(elasticsearchIndicesGetMissingTime, defaultDims, get.MissingTimeInMillis),
		}...)
	}

	out = append(out, []*metricspb.Metric{
		esUtils.PrepareCumulative(elasticsearchIndicesGetTotal, defaultDims, get.Total),
	}...)

	return out
}

func (search *Search) getSearchStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesSearchQueryCurrent, defaultDims, search.QueryCurrent),
			esUtils.PrepareGauge(elasticsearchIndicesSearchFetchCurrent, defaultDims, search.FetchCurrent),
			esUtils.PrepareGauge(elasticsearchIndicesSearchScrollCurrent, defaultDims, search.ScrollCurrent),
			esUtils.PrepareGauge(elasticsearchIndicesSearchSuggestCurrent, defaultDims, search.SuggestCurrent),
			esUtils.PrepareGauge(elasticsearchIndicesSearchOpenContexts, defaultDims, search.SuggestCurrent),
			esUtils.PrepareCumulative(elasticsearchIndicesSearchFetchTime, defaultDims, search.FetchTimeInMillis),
			esUtils.PrepareCumulative(elasticsearchIndicesSearchFetchTotal, defaultDims, search.FetchTotal),
			esUtils.PrepareCumulative(elasticsearchIndicesSearchScrollTime, defaultDims, search.ScrollTimeInMillis),
			esUtils.PrepareCumulative(elasticsearchIndicesSearchScrollTotal, defaultDims, search.ScrollTotal),
			esUtils.PrepareCumulative(elasticsearchIndicesSearchSuggestTime, defaultDims, search.SuggestTimeInMillis),
			esUtils.PrepareCumulative(elasticsearchIndicesSearchSuggestTotal, defaultDims, search.SuggestTotal),
		}...)
	}

	out = append(out, []*metricspb.Metric{
		esUtils.PrepareCumulative(elasticsearchIndicesSearchQueryTime, defaultDims, search.QueryTimeInMillis),
		esUtils.PrepareCumulative(elasticsearchIndicesSearchQueryTotal, defaultDims, search.QueryTotal),
	}...)

	return out
}

func (merges *Merges) getMergesStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesMergesCurrentDocs, defaultDims, merges.CurrentDocs),
			esUtils.PrepareGauge(elasticsearchIndicesMergesCurrentSize, defaultDims, merges.CurrentSizeInBytes),
			esUtils.PrepareCumulative(elasticsearchIndicesMergesTotalDocs, defaultDims, merges.TotalDocs),
			esUtils.PrepareCumulative(elasticsearchIndicesMergesTotalSize, defaultDims, merges.TotalSizeInBytes),
			esUtils.PrepareCumulative(elasticsearchIndicesMergesStoppedTime, defaultDims, merges.TotalStoppedTimeInMillis),
			esUtils.PrepareCumulative(elasticsearchIndicesMergesThrottleTime, defaultDims, merges.TotalThrottledTimeInMillis),
			esUtils.PrepareCumulative(elasticsearchIndicesMergesAutoThrottleSize, defaultDims, merges.TotalAutoThrottleInBytes),
		}...)
	}

	out = append(out, []*metricspb.Metric{
		esUtils.PrepareGauge(elasticsearchIndicesMergesCurrent, defaultDims, merges.Current),
		esUtils.PrepareCumulative(elasticsearchIndicesMergesTotal, defaultDims, merges.Total),
		esUtils.PrepareCumulative(elasticsearchIndicesMergesTotalTime, defaultDims, merges.TotalTimeInMillis),
	}...)

	return out
}

func (refresh *Refresh) getRefreshStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesRefreshListeners, defaultDims, refresh.Listeners),
			esUtils.PrepareCumulative(elasticsearchIndicesRefreshTotal, defaultDims, refresh.Total),
			esUtils.PrepareCumulative(elasticsearchIndicesRefreshTotalTime, defaultDims, refresh.TotalTimeInMillis),
		}...)
	}

	return out
}

func (flush *Flush) getFlushStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesFlushPeriodic, defaultDims, flush.Periodic),
			esUtils.PrepareCumulative(elasticsearchIndicesFlushTotal, defaultDims, flush.Total),
			esUtils.PrepareCumulative(elasticsearchIndicesFlushTotalTime, defaultDims, flush.TotalTimeInMillis),
		}...)
	}

	return out
}

func (warmer *Warmer) getWarmerStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesWarmerCurrent, defaultDims, warmer.Current),
			esUtils.PrepareCumulative(elasticsearchIndicesWarmerTotal, defaultDims, warmer.Total),
			esUtils.PrepareCumulative(elasticsearchIndicesWarmerTotalTime, defaultDims, warmer.TotalTimeInMillis),
		}...)
	}

	return out
}

func (queryCache *QueryCache) getQueryCacheStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesQueryCacheCacheSize, defaultDims, queryCache.CacheSize),
			esUtils.PrepareGauge(elasticsearchIndicesQueryCacheCacheCount, defaultDims, queryCache.CacheCount),
			esUtils.PrepareCumulative(elasticsearchIndicesQueryCacheEvictions, defaultDims, queryCache.Evictions),
			esUtils.PrepareCumulative(elasticsearchIndicesQueryCacheHitCount, defaultDims, queryCache.HitCount),
			esUtils.PrepareCumulative(elasticsearchIndicesQueryCacheMissCount, defaultDims, queryCache.MissCount),
			esUtils.PrepareCumulative(elasticsearchIndicesQueryCacheTotalCount, defaultDims, queryCache.TotalCount),
		}...)
	}
	out = append(out, []*metricspb.Metric{
		esUtils.PrepareGauge(elasticsearchIndicesQueryCacheMemorySize, defaultDims, queryCache.MemorySizeInBytes),
	}...)

	return out
}

func (filterCache *FilterCache) getFilterCacheStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareCumulative(elasticsearchIndicesFilterCacheEvictions, defaultDims, filterCache.Evictions),
		}...)

		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesFilterCacheMemorySize, defaultDims, filterCache.MemorySizeInBytes),
		}...)
	}

	return out
}

func (fielddata *Fielddata) getFielddataStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareCumulative(elasticsearchIndicesFielddataEvictions, defaultDims, fielddata.Evictions),
		}...)
	}

	out = append(out, []*metricspb.Metric{
		esUtils.PrepareGauge(elasticsearchIndicesFielddataMemorySize, defaultDims, fielddata.MemorySizeInBytes),
	}...)

	return out
}

func (completion *Completion) getCompletionStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesCompletionSize, defaultDims, completion.SizeInBytes),
		}...)
	}

	return out
}

func (segments *Segments) getSegmentsStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesSegmentsMemorySize, defaultDims, segments.MemoryInBytes),
			esUtils.PrepareGauge(elasticsearchIndicesSegmentsIndexWriterMemorySize, defaultDims, segments.IndexWriterMemoryInBytes),
			esUtils.PrepareGauge(elasticsearchIndicesSegmentsIndexWriterMaxMemorySize, defaultDims, segments.IndexWriterMaxMemoryInBytes),
			esUtils.PrepareGauge(elasticsearchIndicesSegmentsVersionMapMemorySize, defaultDims, segments.VersionMapMemoryInBytes),
			esUtils.PrepareGauge(elasticsearchIndicesSegmentsTermsMemorySize, defaultDims, segments.TermsMemoryInBytes),
			esUtils.PrepareGauge(elasticsearchIndicesSegmentsStoredFieldMemorySize, defaultDims, segments.StoredFieldsMemoryInBytes),
			esUtils.PrepareGauge(elasticsearchIndicesSegmentsTermVectorsMemorySize, defaultDims, segments.TermVectorsMemoryInBytes),
			esUtils.PrepareGauge(elasticsearchIndicesSegmentsNormsMemorySize, defaultDims, segments.NormsMemoryInBytes),
			esUtils.PrepareGauge(elasticsearchIndicesSegmentsPointsMemorySize, defaultDims, segments.PointsMemoryInBytes),
			esUtils.PrepareGauge(elasticsearchIndicesSegmentsDocValuesMemorySize, defaultDims, segments.DocValuesMemoryInBytes),
			esUtils.PrepareGauge(elasticsearchIndicesSegmentsFixedBitSetMemorySize, defaultDims, segments.FixedBitSetMemoryInBytes),
		}...)
	}

	out = append(out, []*metricspb.Metric{
		esUtils.PrepareGauge(elasticsearchIndicesSegmentsCount, defaultDims, segments.Count),
	}...)

	return out
}

func (translog *Translog) getTranslogStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesTranslogUncommittedOperations, defaultDims, translog.UncommittedOperations),
			esUtils.PrepareGauge(elasticsearchIndicesTranslogUncommittedSizeInBytes, defaultDims, translog.UncommittedSizeInBytes),
			esUtils.PrepareGauge(elasticsearchIndicesTranslogEarliestLastModifiedAge, defaultDims, translog.EarliestLastModifiedAge),
			esUtils.PrepareGauge(elasticsearchIndicesTranslogOperations, defaultDims, translog.Operations),
			esUtils.PrepareGauge(elasticsearchIndicesTranslogSize, defaultDims, translog.SizeInBytes),
		}...)
	}

	return out
}

func (requestCache *RequestCache) getRequestCacheStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareCumulative(elasticsearchIndicesRequestCacheEvictions, defaultDims, requestCache.Evictions),
			esUtils.PrepareCumulative(elasticsearchIndicesRequestCacheHitCount, defaultDims, requestCache.HitCount),
			esUtils.PrepareCumulative(elasticsearchIndicesRequestCacheMissCount, defaultDims, requestCache.MissCount),
		}...)
	}

	out = append(out, esUtils.PrepareGauge(elasticsearchIndicesRequestCacheMemorySize, defaultDims, requestCache.MemorySizeInBytes))

	return out
}

func (recovery *Recovery) getRecoveryStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesRecoveryCurrentAsSource, defaultDims, recovery.CurrentAsSource),
			esUtils.PrepareGauge(elasticsearchIndicesRecoveryCurrentAsTarget, defaultDims, recovery.CurrentAsTarget),
			esUtils.PrepareCumulative(elasticsearchIndicesRecoveryThrottleTime, defaultDims, recovery.ThrottleTimeInMillis),
		}...)
	}

	return out
}

func (idCache *IDCache) getIDCacheStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesIDCacheMemorySize, defaultDims, idCache.MemorySizeInBytes),
		}...)
	}

	return out
}

func (suggest *Suggest) getSuggestStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesSuggestCurrent, defaultDims, suggest.Current),
			esUtils.PrepareCumulative(elasticsearchIndicesSuggestTime, defaultDims, suggest.TimeInMillis),
			esUtils.PrepareCumulative(elasticsearchIndicesSuggestTotal, defaultDims, suggest.Total),
		}...)
	}

	return out
}

func (percolate *Percolate) getPercolateStats(enhanced bool, defaultDims map[string]string) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			esUtils.PrepareGauge(elasticsearchIndicesPercolateCurrent, defaultDims, percolate.Current),
			esUtils.PrepareCumulative(elasticsearchIndicesPercolateTotal, defaultDims, percolate.Total),
			esUtils.PrepareCumulative(elasticsearchIndicesPercolateQueries, defaultDims, percolate.Queries),
			esUtils.PrepareCumulative(elasticsearchIndicesPercolateTime, defaultDims, percolate.TimeInMillis),
		}...)
	}

	return out
}
