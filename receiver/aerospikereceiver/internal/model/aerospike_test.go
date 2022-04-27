package model_test

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/internal/model"
	"github.com/stretchr/testify/require"
)

func TestParseInfo(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		input    model.InfoResponse
		expected *model.NodeInfo
	}{
		{
			name:     "empty response",
			input:    make(model.InfoResponse),
			expected: &model.NodeInfo{},
		},
		{
			name: "missing statistics",
			input: model.InfoResponse{
				"node":       "missing statistics",
				"namespaces": "test;bar",
				"services":   "0.0.0.0:3002;0.0.0.1:3002",
				"not-stats":  "k=v;k1=v1",
			},
			expected: &model.NodeInfo{
				Name:       "missing statistics",
				Namespaces: []string{"test", "bar"},
				Services:   []string{"0.0.0.0:3002", "0.0.0.1:3002"},
			},
		},
		{
			name: "malformed statistics value",
			input: model.InfoResponse{
				"node":       "malformed value",
				"namespaces": "test;bar",
				"services":   "0.0.0.0:3002;0.0.0.1:3002",
				"statistics": "client_connections=1=1;client_connections_closed=32",
			},
			expected: &model.NodeInfo{
				Name:       "malformed value",
				Namespaces: []string{"test", "bar"},
				Services:   []string{"0.0.0.0:3002", "0.0.0.1:3002"},
				Statistics: &model.NodeStats{
					ClientConnectionsClosed: intPtr(32),
				},
			},
		},
		{
			name: "full response",
			input: model.InfoResponse{
				"node":       "plain statistics",
				"namespaces": "ns1;ns2;last-namespace",
				"services":   "0.0.0.0:3002;0.0.0.1:3002",
				"statistics": "failed_best_practices=true;cluster_size=1;cluster_key=D26D32A9FC89;cluster_generation=1;cluster_principal=BB9030011AC4202;cluster_min_compatibility_id=10;cluster_max_compatibility_id=10;cluster_integrity=true;cluster_is_member=true;cluster_duplicate_nodes=null;cluster_clock_skew_stop_writes_sec=0;cluster_clock_skew_ms=0;cluster_clock_skew_outliers=null;uptime=15;system_total_cpu_pct=10;system_user_cpu_pct=7;system_kernel_cpu_pct=3;system_free_mem_kbytes=3411332;system_free_mem_pct=84;system_thp_mem_kbytes=456704;process_cpu_pct=1;threads_joinable=9;threads_detached=61;threads_pool_total=42;threads_pool_active=42;heap_allocated_kbytes=1113069;heap_active_kbytes=1113568;heap_mapped_kbytes=1143296;heap_efficiency_pct=97;heap_site_count=0;objects=75183;tombstones=0;info_queue=0;rw_in_progress=0;proxy_in_progress=0;tree_gc_queue=0;client_connections=1;client_connections_opened=19;client_connections_closed=18;heartbeat_connections=0;heartbeat_connections_opened=0;heartbeat_connections_closed=0;fabric_connections=20;fabric_connections_opened=39;fabric_connections_closed=19;heartbeat_received_self=0;heartbeat_received_foreign=0;reaped_fds=0;info_complete=0;demarshal_error=0;early_tsvc_client_error=0;early_tsvc_from_proxy_error=0;early_tsvc_batch_sub_error=0;early_tsvc_from_proxy_batch_sub_error=0;early_tsvc_udf_sub_error=0;early_tsvc_ops_sub_error=0;batch_index_initiate=0;batch_index_queue=0:0,0:0;batch_index_complete=0;batch_index_error=0;batch_index_timeout=0;batch_index_delay=0;batch_index_unused_buffers=0;batch_index_huge_buffers=0;batch_index_created_buffers=0;batch_index_destroyed_buffers=0;batch_index_proto_uncompressed_pct=0.000;batch_index_proto_compression_ratio=1.000;scans_active=0;query_short_running=0;query_long_running=0;paxos_principal=BB9030011AC4202;time_since_rebalance=13;migrate_allowed=true;migrate_partitions_remaining=0;fabric_bulk_send_rate=0;fabric_bulk_recv_rate=0;fabric_ctrl_send_rate=0;fabric_ctrl_recv_rate=0;fabric_meta_send_rate=0;fabric_meta_recv_rate=0;fabric_rw_send_rate=0;fabric_rw_recv_rate=0"},
			expected: &model.NodeInfo{
				Name:       "plain statistics",
				Namespaces: []string{"ns1", "ns2", "last-namespace"},
				Services:   []string{"0.0.0.0:3002", "0.0.0.1:3002"},
				Statistics: &model.NodeStats{
					ClientConnections:          intPtr(1),
					ClientConnectionsClosed:    intPtr(18),
					ClientConnectionsOpened:    intPtr(19),
					FabricConnections:          intPtr(20),
					FabricConnectionsClosed:    intPtr(19),
					FabricConnectionsOpened:    intPtr(39),
					HeartbeatConnections:       intPtr(0),
					HeartbeatConnectionsClosed: intPtr(0),
					HeartbeatConnectionsOpened: intPtr(0),
					SystemFreeMemPct:           intPtr(84),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stats := model.ParseInfo(tc.input)
			require.Equal(t, tc.expected, stats)
		})
	}
}

func TestParseNamespaceInfo(t *testing.T) {
	const namespace = "test"

	testCases := []struct {
		name     string
		input    model.InfoResponse
		expected *model.NamespaceInfo
	}{
		{
			name:  "empty response",
			input: model.InfoResponse{},
			expected: &model.NamespaceInfo{
				Name: namespace,
			},
		},
		{
			name: "missing key",
			input: model.InfoResponse{
				namespace + "bad": "",
			},
			expected: &model.NamespaceInfo{
				Name: namespace,
			},
		},
		{
			name: "regular info",
			input: model.InfoResponse{
				model.NamespaceKey(namespace): "ns_cluster_size=3;effective_replication_factor=2;objects=504464;tombstones=0;xdr_tombstones=0;xdr_bin_cemeteries=0;master_objects=246470;master_tombstones=0;prole_objects=257994;prole_tombstones=0;non_replica_objects=0;non_replica_tombstones=0;unreplicated_records=0;dead_partitions=0;unavailable_partitions=0;clock_skew_stop_writes=false;stop_writes=false;hwm_breached=false;current_time=388615645;non_expirable_objects=0;expired_objects=0;evicted_objects=0;evict_ttl=0;evict_void_time=0;smd_evict_void_time=0;nsup_cycle_duration=0;truncate_lut=0;truncated_records=0;sindex_gc_cleaned=0;memory_used_bytes=38961748;memory_used_data_bytes=6676052;memory_used_index_bytes=32285696;memory_used_set_index_bytes=0;memory_used_sindex_bytes=0;memory_free_pct=96;xmem_id=0;available_bin_names=65532;device_total_bytes=4294967296;device_used_bytes=32395504;device_free_pct=99;device_available_pct=98;storage-engine.file[0].used_bytes=32395504;storage-engine.file[0].free_wblocks=4044;storage-engine.file[0].write_q=0;storage-engine.file[0].writes=0;storage-engine.file[0].defrag_q=0;storage-engine.file[0].defrag_reads=3;storage-engine.file[0].defrag_writes=1;storage-engine.file[0].shadow_write_q=0;storage-engine.file[0].age=-1;record_proto_uncompressed_pct=0.000;record_proto_compression_ratio=1.000;scan_proto_uncompressed_pct=0.000;scan_proto_compression_ratio=1.000;query_proto_uncompressed_pct=0.000;query_proto_compression_ratio=1.000;pending_quiesce=false;effective_is_quiesced=false;nodes_quiesced=0;effective_prefer_uniform_balance=false;migrate_tx_partitions_imbalance=0;migrate_tx_instances=1396;migrate_rx_instances=1102;migrate_tx_partitions_active=1;migrate_rx_partitions_active=1;migrate_tx_partitions_initial=2711;migrate_tx_partitions_remaining=2295;migrate_tx_partitions_lead_remaining=951;migrate_rx_partitions_initial=3408;migrate_rx_partitions_remaining=2307;migrate_records_skipped=0;migrate_records_transmitted=76523;migrate_record_retransmits=0;migrate_record_receives=74858;migrate_signals_active=0;migrate_signals_remaining=2688;appeals_tx_active=0;appeals_rx_active=0;appeals_tx_remaining=0;appeals_records_exonerated=0;client_tsvc_error=0;client_tsvc_timeout=0;client_proxy_complete=0;client_proxy_error=0;client_proxy_timeout=0;client_read_success=6085;client_read_error=0;client_read_timeout=0;client_read_not_found=1980;client_read_filtered_out=0;client_write_success=7718;client_write_error=0;client_write_timeout=0;client_write_filtered_out=0;xdr_client_write_success=0;xdr_client_write_error=0;xdr_client_write_timeout=0;client_delete_success=0;client_delete_error=0;client_delete_timeout=0;client_delete_not_found=0;client_delete_filtered_out=0;xdr_client_delete_success=0;xdr_client_delete_error=0;xdr_client_delete_timeout=0;xdr_client_delete_not_found=0;client_udf_complete=0;client_udf_error=0;client_udf_timeout=0;client_udf_filtered_out=0;client_lang_read_success=0;client_lang_write_success=0;client_lang_delete_success=0;client_lang_error=0;from_proxy_tsvc_error=0;from_proxy_tsvc_timeout=0;from_proxy_read_success=0;from_proxy_read_error=0;from_proxy_read_timeout=0;from_proxy_read_not_found=0;from_proxy_read_filtered_out=0;from_proxy_write_success=0;from_proxy_write_error=0;from_proxy_write_timeout=0;from_proxy_write_filtered_out=0;xdr_from_proxy_write_success=0;xdr_from_proxy_write_error=0;xdr_from_proxy_write_timeout=0;from_proxy_delete_success=0;from_proxy_delete_error=0;from_proxy_delete_timeout=0;from_proxy_delete_not_found=0;from_proxy_delete_filtered_out=0;xdr_from_proxy_delete_success=0;xdr_from_proxy_delete_error=0;xdr_from_proxy_delete_timeout=0;xdr_from_proxy_delete_not_found=0;from_proxy_udf_complete=0;from_proxy_udf_error=0;from_proxy_udf_timeout=0;from_proxy_udf_filtered_out=0;from_proxy_lang_read_success=0;from_proxy_lang_write_success=0;from_proxy_lang_delete_success=0;from_proxy_lang_error=0;batch_sub_tsvc_error=0;batch_sub_tsvc_timeout=0;batch_sub_proxy_complete=0;batch_sub_proxy_error=0;batch_sub_proxy_timeout=0;batch_sub_read_success=0;batch_sub_read_error=0;batch_sub_read_timeout=0;batch_sub_read_not_found=0;batch_sub_read_filtered_out=0;from_proxy_batch_sub_tsvc_error=0;from_proxy_batch_sub_tsvc_timeout=0;from_proxy_batch_sub_read_success=0;from_proxy_batch_sub_read_error=0;from_proxy_batch_sub_read_timeout=0;from_proxy_batch_sub_read_not_found=0;from_proxy_batch_sub_read_filtered_out=0;udf_sub_tsvc_error=0;udf_sub_tsvc_timeout=0;udf_sub_udf_complete=0;udf_sub_udf_error=0;udf_sub_udf_timeout=0;udf_sub_udf_filtered_out=0;udf_sub_lang_read_success=0;udf_sub_lang_write_success=0;udf_sub_lang_delete_success=0;udf_sub_lang_error=0;ops_sub_tsvc_error=0;ops_sub_tsvc_timeout=0;ops_sub_write_success=0;ops_sub_write_error=0;ops_sub_write_timeout=0;ops_sub_write_filtered_out=0;dup_res_ask=5539;dup_res_respond_read=0;dup_res_respond_no_read=5966;retransmit_all_read_dup_res=0;retransmit_all_write_dup_res=0;retransmit_all_write_repl_write=0;retransmit_all_delete_dup_res=0;retransmit_all_delete_repl_write=0;retransmit_all_udf_dup_res=0;retransmit_all_udf_repl_write=0;retransmit_all_batch_sub_dup_res=0;retransmit_udf_sub_dup_res=0;retransmit_udf_sub_repl_write=0;retransmit_ops_sub_dup_res=0;retransmit_ops_sub_repl_write=0;scan_basic_complete=12;scan_basic_error=22;scan_basic_abort=1;scan_aggr_complete=9;scan_aggr_error=1;scan_aggr_abort=3;scan_udf_bg_complete=40;scan_udf_bg_error=4;scan_udf_bg_abort=14;scan_ops_bg_complete=30;scan_ops_bg_error=3;scan_ops_bg_abort=13;query_reqs=0;query_fail=0;query_false_positives=0;query_short_queue_full=0;query_long_queue_full=0;query_short_reqs=0;query_long_reqs=0;query_basic_complete=0;query_basic_error=0;query_basic_abort=0;query_basic_avg_rec_count=0;query_aggr_complete=0;query_aggr_error=0;query_aggr_abort=0;query_aggr_avg_rec_count=0;query_udf_bg_complete=0;query_udf_bg_error=0;query_udf_bg_abort=0;query_ops_bg_complete=0;query_ops_bg_error=0;query_ops_bg_abort=0;geo_region_query_reqs=0;geo_region_query_cells=0;geo_region_query_points=0;geo_region_query_falsepos=0;re_repl_success=0;re_repl_error=0;re_repl_timeout=0;fail_xdr_forbidden=0;fail_key_busy=0;fail_generation=0;fail_record_too_big=0;fail_client_lost_conflict=0;fail_xdr_lost_conflict=0;deleted_last_bin=0;replication-factor=2;memory-size=1073741824;default-ttl=2592000;allow-ttl-without-nsup=false;background-scan-max-rps=10000;conflict-resolution-policy=generation;conflict-resolve-writes=false;data-in-index=false;disable-cold-start-eviction=false;disable-write-dup-res=false;disallow-null-setname=false;enable-benchmarks-batch-sub=false;enable-benchmarks-ops-sub=false;enable-benchmarks-read=false;enable-benchmarks-udf=false;enable-benchmarks-udf-sub=false;enable-benchmarks-write=false;enable-hist-proxy=false;evict-hist-buckets=10000;evict-tenths-pct=5;high-water-disk-pct=15;high-water-memory-pct=5;ignore-migrate-fill-delay=false;index-stage-size=1073741824;index-type=mem;max-record-size=0;migrate-order=5;migrate-retransmit-ms=5000;migrate-sleep=1;nsup-hist-period=3600;nsup-period=120;nsup-threads=1;partition-tree-sprigs=256;prefer-uniform-balance=false;rack-id=0;read-consistency-level-override=off;reject-non-xdr-writes=false;reject-xdr-writes=false;single-bin=false;single-scan-threads=4;stop-writes-pct=90;strong-consistency=false;strong-consistency-allow-expunge=false;tomb-raider-eligible-age=86400;tomb-raider-period=86400;transaction-pending-limit=20;truncate-threads=4;write-commit-level-override=off;xdr-bin-tombstone-ttl=86400;xdr-tomb-raider-period=120;xdr-tomb-raider-threads=1;storage-engine=device;storage-engine.file[0]=/opt/aerospike/data/test.dat;storage-engine.filesize=4294967296;storage-engine.scheduler-mode=null;storage-engine.write-block-size=1048576;storage-engine.data-in-memory=true;storage-engine.cache-replica-writes=false;storage-engine.cold-start-empty=false;storage-engine.commit-to-device=false;storage-engine.commit-min-size=0;storage-engine.compression=none;storage-engine.compression-level=0;storage-engine.defrag-lwm-pct=50;storage-engine.defrag-queue-min=0;storage-engine.defrag-sleep=1000;storage-engine.defrag-startup-minimum=0;storage-engine.direct-files=false;storage-engine.disable-odsync=false;storage-engine.enable-benchmarks-storage=false;storage-engine.encryption-key-file=null;storage-engine.encryption-old-key-file=null;storage-engine.flush-max-ms=1000;storage-engine.max-write-cache=67108864;storage-engine.min-avail-pct=5;storage-engine.post-write-queue=0;storage-engine.read-page-cache=false;storage-engine.serialize-tomb-raider=false;storage-engine.sindex-startup-device-scan=false;storage-engine.tomb-raider-sleep=1000;sindex.num-partitions=32;geo2dsphere-within.strict=true;geo2dsphere-within.min-level=1;geo2dsphere-within.max-level=20;geo2dsphere-within.max-cells=12;geo2dsphere-within.level-mod=1;geo2dsphere-within.earth-radius-meters=6371000",
			},
			expected: &model.NamespaceInfo{
				Name:                    namespace,
				DeviceAvailablePct:      intPtr(98),
				HighWaterDiskPct:        intPtr(15),
				DeviceUsedBytes:         intPtr(32395504),
				MemoryFreePct:           intPtr(96),
				HighWaterMemoryPct:      intPtr(5),
				MemoryUsedDataBytes:     intPtr(6676052),
				MemoryUsedIndexBytes:    intPtr(32285696),
				MemoryUsedSIndexBytes:   intPtr(0),
				MemoryUsedSetIndexBytes: intPtr(0),
				ScanAggrAbort:           intPtr(3),
				ScanAggrComplete:        intPtr(9),
				ScanAggrError:           intPtr(1),
				ScanBasicAbort:          intPtr(1),
				ScanBasicComplete:       intPtr(12),
				ScanBasicError:          intPtr(22),
				ScanOpsBgAbort:          intPtr(13),
				ScanOpsBgComplete:       intPtr(30),
				ScanOpsBgError:          intPtr(3),
				ScanUdfBgAbort:          intPtr(14),
				ScanUdfBgComplete:       intPtr(40),
				ScanUdfBgError:          intPtr(4),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			namespaceInfo := model.ParseNamespaceInfo(tc.input, namespace)
			require.Equal(t, tc.expected, namespaceInfo)
		})
	}
}

// intPtr returns a pointer to the given int
func intPtr(v int64) *int64 {
	return &v
}
