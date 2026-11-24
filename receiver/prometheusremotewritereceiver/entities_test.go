// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/gogo/protobuf/proto"
	remoteapi "github.com/prometheus/client_golang/exp/api/remote"
	"github.com/prometheus/prometheus/model/value"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/xpdata/entity"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

type prwTestSeries struct {
	labels    map[string]string
	mtype     writev2.Metadata_MetricType
	samples   []writev2.Sample
	exemplars []writev2.Exemplar
}

func buildPRWV2Request(series ...prwTestSeries) *writev2.Request {
	symTable := writev2.NewSymbolTable()
	req := &writev2.Request{
		Timeseries: make([]writev2.TimeSeries, 0, len(series)),
	}
	for _, s := range series {
		keys := make([]string, 0, len(s.labels))
		for k := range s.labels {
			keys = append(keys, k)
		}
		slices.Sort(keys)

		lblRefs := make([]uint32, 0, len(keys)*2)
		for _, k := range keys {
			lblRefs = append(lblRefs, symTable.Symbolize(k), symTable.Symbolize(s.labels[k]))
		}
		req.Timeseries = append(req.Timeseries, writev2.TimeSeries{
			LabelsRefs: lblRefs,
			Metadata:   writev2.Metadata{Type: s.mtype},
			Samples:    s.samples,
			Exemplars:  s.exemplars,
		})
	}
	req.Symbols = symTable.Symbols()
	return req
}

func postPRWV2(t *testing.T, r *prometheusRemoteWriteReceiver, req *writev2.Request) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := proto.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest(http.MethodPost, writePath, bytes.NewReader(payload))
	httpReq.Header.Set("Content-Type", fmt.Sprintf("application/x-protobuf;proto=%s", remoteapi.WriteV2MessageType))
	rec := httptest.NewRecorder()
	r.handlePRW(rec, httpReq)
	return rec
}

func TestTranslateV2EntityInfoSingleRequest(t *testing.T) {
	metricTypes := []struct {
		name  string
		mtype writev2.Metadata_MetricType
	}{
		{name: "info", mtype: writev2.Metadata_METRIC_TYPE_INFO},
		{name: "gauge", mtype: writev2.Metadata_METRIC_TYPE_GAUGE},
		{name: "unspecified", mtype: writev2.Metadata_METRIC_TYPE_UNSPECIFIED},
	}

	for _, mt := range metricTypes {
		for _, entityFirst := range []bool{true, false} {
			order := "target_first"
			if entityFirst {
				order = "entity_first"
			}
			t.Run(mt.name+"/"+order, func(t *testing.T) {
				r := setupMetricsReceiver(t)

				targetSeries := prwTestSeries{
					labels: map[string]string{
						"__name__":            "target_info",
						"job":                 "shop/checkout",
						"instance":            "id-1",
						"service_namespace":   "shop",
						"service_name":        "checkout",
						"service_instance_id": "id-1",
						"service_version":     "2.4.0",
						"k8s_pod_uid":         "pod-uid-123",
						"k8s_pod_name":        "checkout-7f9",
					},
					mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
					samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
				}
				svcEntitySeries := prwTestSeries{
					labels: map[string]string{
						"__name__":                                prometheus.EntityInfoMetricName,
						"job":                                     "shop/checkout",
						"instance":                                "id-1",
						prometheus.EntityTypeLabelKey:             "service.instance",
						prometheus.EntitySchemaURLLabelKey:        "https://opentelemetry.io/schemas/1.31.0",
						prometheus.EntityIDKeysLabelKey:           `["service_instance_id","service_name","service_namespace"]`,
						prometheus.EntityDescriptionKeysLabelKey:  `["service_version","missing_desc"]`,
						"otel_entity_future_unrecognized_feature": "ignored",
					},
					mtype:   mt.mtype,
					samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
				}
				podEntitySeries := prwTestSeries{
					labels: map[string]string{
						"__name__":                               prometheus.EntityInfoMetricName,
						"job":                                    "shop/checkout",
						"instance":                               "id-1",
						prometheus.EntityTypeLabelKey:            "k8s.pod",
						prometheus.EntityIDKeysLabelKey:          `["k8s_pod_uid"]`,
						prometheus.EntityDescriptionKeysLabelKey: `["k8s_pod_name"]`,
					},
					mtype:   mt.mtype,
					samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
				}
				malformedEntitySeries := prwTestSeries{
					labels: map[string]string{
						"__name__":                      prometheus.EntityInfoMetricName,
						"job":                           "shop/checkout",
						"instance":                      "id-1",
						prometheus.EntityTypeLabelKey:   "broken",
						prometheus.EntityIDKeysLabelKey: "not-json",
					},
					mtype:   mt.mtype,
					samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
				}
				missingIDEntitySeries := prwTestSeries{
					labels: map[string]string{
						"__name__":                      prometheus.EntityInfoMetricName,
						"job":                           "shop/checkout",
						"instance":                      "id-1",
						prometheus.EntityTypeLabelKey:   "host",
						prometheus.EntityIDKeysLabelKey: `["host_id"]`,
					},
					mtype:   mt.mtype,
					samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
				}
				regularMetric := prwTestSeries{
					labels: map[string]string{
						"__name__": "http_requests",
						"job":      "shop/checkout",
						"instance": "id-1",
					},
					mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
					samples: []writev2.Sample{{Value: 42, Timestamp: 1000}},
				}

				var req *writev2.Request
				if entityFirst {
					req = buildPRWV2Request(svcEntitySeries, podEntitySeries, malformedEntitySeries, missingIDEntitySeries, targetSeries, regularMetric)
				} else {
					req = buildPRWV2Request(targetSeries, regularMetric, svcEntitySeries, podEntitySeries, malformedEntitySeries, missingIDEntitySeries)
				}

				md, stats, err := r.translateV2(t.Context(), req)
				require.NoError(t, err)
				require.Equal(t, 6, stats.Samples, "all target_info, otel_entity_info, and metric samples must be counted")
				require.Equal(t, 1, md.ResourceMetrics().Len())

				rm := md.ResourceMetrics().At(0)
				// Only http_requests should remain as a metric.
				require.Equal(t, 1, rm.ScopeMetrics().Len())
				require.Equal(t, 1, rm.ScopeMetrics().At(0).Metrics().Len())
				require.Equal(t, "http_requests", rm.ScopeMetrics().At(0).Metrics().At(0).Name())

				refs := entity.ResourceEntityRefs(rm.Resource())
				require.Equal(t, 2, refs.Len())
				// Order must be deterministic and match request order when timestamps are equal.
				require.Equal(t, "service.instance", refs.At(0).Type())
				require.Equal(t, "https://opentelemetry.io/schemas/1.31.0", refs.At(0).SchemaUrl())
				require.Equal(t, []string{"service_instance_id", "service_name", "service_namespace"}, refs.At(0).IdKeys().AsRaw())
				require.Equal(t, []string{"service_version"}, refs.At(0).DescriptionKeys().AsRaw())

				require.Equal(t, "k8s.pod", refs.At(1).Type())
				require.Empty(t, refs.At(1).SchemaUrl())
				require.Equal(t, []string{"k8s_pod_uid"}, refs.At(1).IdKeys().AsRaw())
				require.Equal(t, []string{"k8s_pod_name"}, refs.At(1).DescriptionKeys().AsRaw())
			})
		}
	}
}

func TestTranslateV2EntityInfoCounterTypeNotDropped(t *testing.T) {
	r := setupMetricsReceiver(t)
	req := buildPRWV2Request(prwTestSeries{
		labels: map[string]string{
			"__name__":                      prometheus.EntityInfoMetricName,
			"job":                           "svc",
			"instance":                      "inst-1",
			prometheus.EntityTypeLabelKey:   "k8s.pod",
			prometheus.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
		},
		mtype:   writev2.Metadata_METRIC_TYPE_COUNTER,
		samples: []writev2.Sample{{Value: 5, Timestamp: 1000}},
	})

	md, stats, err := r.translateV2(t.Context(), req)
	require.NoError(t, err)
	require.Equal(t, 1, stats.Samples)
	require.Equal(t, 1, md.ResourceMetrics().Len())
	rm := md.ResourceMetrics().At(0)
	require.Equal(t, 0, entity.ResourceEntityRefs(rm.Resource()).Len())
	require.Equal(t, 1, rm.ScopeMetrics().Len())
	require.Equal(t, prometheus.EntityInfoMetricName, rm.ScopeMetrics().At(0).Metrics().At(0).Name())
}

func TestTranslateV2EntityInfoCachingReplacementAndStaleEviction(t *testing.T) {
	r := setupMetricsReceiver(t)

	// Request 1: otel_entity_info arrives BEFORE target_info and metrics.
	// Must produce 0 ResourceMetrics while caching entities for future requests.
	req1 := buildPRWV2Request(
		prwTestSeries{
			labels: map[string]string{
				"__name__":                               prometheus.EntityInfoMetricName,
				"job":                                    "shop/checkout",
				"instance":                               "id-1",
				prometheus.EntityTypeLabelKey:            "k8s.pod",
				prometheus.EntityIDKeysLabelKey:          `["k8s_pod_uid"]`,
				prometheus.EntityDescriptionKeysLabelKey: `["k8s_pod_name","k8s_pod_ip"]`,
			},
			mtype:   writev2.Metadata_METRIC_TYPE_INFO,
			samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
		},
		prwTestSeries{
			labels: map[string]string{
				"__name__":                      prometheus.EntityInfoMetricName,
				"job":                           "shop/checkout",
				"instance":                      "id-1",
				prometheus.EntityTypeLabelKey:   "host",
				prometheus.EntityIDKeysLabelKey: `["host_id"]`,
			},
			mtype:   writev2.Metadata_METRIC_TYPE_INFO,
			samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
		},
	)
	md1, stats1, err := r.translateV2(t.Context(), req1)
	require.NoError(t, err)
	require.Equal(t, 2, stats1.Samples)
	require.Equal(t, 0, md1.ResourceMetrics().Len(), "otel_entity_info alone must not emit empty ResourceMetrics")
	require.Equal(t, 1, r.rmCache.Len())

	// Request 2: target_info + metric arrives -> cached entities now resolve against resource attributes.
	req2 := buildPRWV2Request(
		prwTestSeries{
			labels: map[string]string{
				"__name__":     "target_info",
				"job":          "shop/checkout",
				"instance":     "id-1",
				"k8s_pod_uid":  "pod-1",
				"k8s_pod_name": "checkout-a",
				"k8s_pod_ip":   "10.0.0.1",
				"host_id":      "host-1",
			},
			mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
			samples: []writev2.Sample{{Value: 1, Timestamp: 2000}},
		},
		prwTestSeries{
			labels: map[string]string{
				"__name__": "http_requests",
				"job":      "shop/checkout",
				"instance": "id-1",
			},
			mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
			samples: []writev2.Sample{{Value: 1, Timestamp: 2000}},
		},
	)
	md2, _, err := r.translateV2(t.Context(), req2)
	require.NoError(t, err)
	require.Equal(t, 1, md2.ResourceMetrics().Len())
	refs2 := entity.ResourceEntityRefs(md2.ResourceMetrics().At(0).Resource())
	require.Equal(t, 2, refs2.Len())

	// Request 3: current-request k8s.pod replaces cached k8s.pod (dropping k8s_pod_ip).
	// host entity must remain cached.
	req3 := buildPRWV2Request(
		prwTestSeries{
			labels: map[string]string{
				"__name__":                               prometheus.EntityInfoMetricName,
				"job":                                    "shop/checkout",
				"instance":                               "id-1",
				prometheus.EntityTypeLabelKey:            "k8s.pod",
				prometheus.EntityIDKeysLabelKey:          `["k8s_pod_uid"]`,
				prometheus.EntityDescriptionKeysLabelKey: `["k8s_pod_name"]`,
			},
			mtype:   writev2.Metadata_METRIC_TYPE_INFO,
			samples: []writev2.Sample{{Value: 1, Timestamp: 3000}},
		},
		prwTestSeries{
			labels: map[string]string{
				"__name__": "http_requests",
				"job":      "shop/checkout",
				"instance": "id-1",
			},
			mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
			samples: []writev2.Sample{{Value: 2, Timestamp: 3000}},
		},
	)
	md3, _, err := r.translateV2(t.Context(), req3)
	require.NoError(t, err)
	refs3 := entity.ResourceEntityRefs(md3.ResourceMetrics().At(0).Resource())
	require.Equal(t, 2, refs3.Len())
	byType3 := map[string]entity.EntityRef{}
	for i := 0; i < refs3.Len(); i++ {
		byType3[refs3.At(i).Type()] = refs3.At(i)
	}
	require.Equal(t, []string{"k8s_pod_name"}, byType3["k8s.pod"].DescriptionKeys().AsRaw(), "current-request sample must replace cached entry, removing dropped description key")
	require.Equal(t, []string{"host_id"}, byType3["host"].IdKeys().AsRaw())

	// Request 4: StaleNaN evicts cached host entity; subsequent metric sees only k8s.pod.
	req4 := buildPRWV2Request(
		prwTestSeries{
			labels: map[string]string{
				"__name__":                      prometheus.EntityInfoMetricName,
				"job":                           "shop/checkout",
				"instance":                      "id-1",
				prometheus.EntityTypeLabelKey:   "host",
				prometheus.EntityIDKeysLabelKey: `["host_id"]`,
			},
			mtype:   writev2.Metadata_METRIC_TYPE_INFO,
			samples: []writev2.Sample{{Value: math.Float64frombits(value.StaleNaN), Timestamp: 4000}},
		},
		prwTestSeries{
			labels: map[string]string{
				"__name__": "http_requests",
				"job":      "shop/checkout",
				"instance": "id-1",
			},
			mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
			samples: []writev2.Sample{{Value: 3, Timestamp: 4000}},
		},
	)
	md4, stats4, err := r.translateV2(t.Context(), req4)
	require.NoError(t, err)
	require.Equal(t, 2, stats4.Samples)
	refs4 := entity.ResourceEntityRefs(md4.ResourceMetrics().At(0).Resource())
	require.Equal(t, 1, refs4.Len())
	require.Equal(t, "k8s.pod", refs4.At(0).Type())

	// Request 5: metric-only request still reuses cached attributes and remaining k8s.pod entity.
	req5 := buildPRWV2Request(prwTestSeries{
		labels: map[string]string{
			"__name__": "http_requests",
			"job":      "shop/checkout",
			"instance": "id-1",
		},
		mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
		samples: []writev2.Sample{{Value: 4, Timestamp: 5000}},
	})
	md5, _, err := r.translateV2(t.Context(), req5)
	require.NoError(t, err)
	refs5 := entity.ResourceEntityRefs(md5.ResourceMetrics().At(0).Resource())
	require.Equal(t, 1, refs5.Len())
	require.Equal(t, "k8s.pod", refs5.At(0).Type())
	require.Equal(t, []string{"k8s_pod_name"}, refs5.At(0).DescriptionKeys().AsRaw())
}

func TestHandlePRWEntityInfoFailureAndEdgeCases(t *testing.T) {
	t.Run("failed_consume_does_not_cache_entities", func(t *testing.T) {
		r := setupMetricsReceiver(t)
		r.nextConsumer = consumertest.NewErr(errors.New("downstream unavailable"))

		req := buildPRWV2Request(
			prwTestSeries{
				labels: map[string]string{
					"__name__":    "target_info",
					"job":         "svc",
					"instance":    "1",
					"k8s_pod_uid": "pod-1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__":                      prometheus.EntityInfoMetricName,
					"job":                           "svc",
					"instance":                      "1",
					prometheus.EntityTypeLabelKey:   "k8s.pod",
					prometheus.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
				},
				mtype:   writev2.Metadata_METRIC_TYPE_INFO,
				samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__": "req_total",
					"job":      "svc",
					"instance": "1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
			},
		)

		rec := postPRWV2(t, r, req)
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.Equal(t, 0, r.rmCache.Len())
	})

	t.Run("stale_only_uncached_does_not_populate_cache", func(t *testing.T) {
		r := setupMetricsReceiver(t)
		req := buildPRWV2Request(prwTestSeries{
			labels: map[string]string{
				"__name__":                      prometheus.EntityInfoMetricName,
				"job":                           "svc",
				"instance":                      "1",
				prometheus.EntityTypeLabelKey:   "k8s.pod",
				prometheus.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
			},
			mtype:   writev2.Metadata_METRIC_TYPE_INFO,
			samples: []writev2.Sample{{Value: math.Float64frombits(value.StaleNaN), Timestamp: 1000}},
		})
		rec := postPRWV2(t, r, req)
		require.Equal(t, http.StatusNoContent, rec.Code)
		require.Equal(t, "1", rec.Header().Get("X-Prometheus-Remote-Write-Samples-Written"))
		require.Equal(t, 0, r.rmCache.Len())
	})

	t.Run("exemplar_only_entity_info_does_not_mutate_cache", func(t *testing.T) {
		r := setupMetricsReceiver(t)
		req := buildPRWV2Request(prwTestSeries{
			labels: map[string]string{
				"__name__":                      prometheus.EntityInfoMetricName,
				"job":                           "svc",
				"instance":                      "1",
				prometheus.EntityTypeLabelKey:   "k8s.pod",
				prometheus.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
			},
			mtype:     writev2.Metadata_METRIC_TYPE_INFO,
			exemplars: []writev2.Exemplar{{Value: 1, Timestamp: 1000}},
		})
		rec := postPRWV2(t, r, req)
		require.Equal(t, http.StatusNoContent, rec.Code)
		require.Equal(t, 0, r.rmCache.Len())
	})

	t.Run("invalid_id_keys_does_not_evict_valid_cached_entity", func(t *testing.T) {
		r := setupMetricsReceiver(t)

		// Request 1 caches target_info and valid k8s.pod entity.
		req1 := buildPRWV2Request(
			prwTestSeries{
				labels: map[string]string{
					"__name__":    "target_info",
					"job":         "svc",
					"instance":    "1",
					"k8s_pod_uid": "pod-1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__":                      prometheus.EntityInfoMetricName,
					"job":                           "svc",
					"instance":                      "1",
					prometheus.EntityTypeLabelKey:   "k8s.pod",
					prometheus.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
				},
				mtype:   writev2.Metadata_METRIC_TYPE_INFO,
				samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
			},
		)
		_, _, err := r.translateV2(t.Context(), req1)
		require.NoError(t, err)

		// Request 2 sends an invalid k8s.pod entity (missing_key not in resource attrs) + metric.
		req2 := buildPRWV2Request(
			prwTestSeries{
				labels: map[string]string{
					"__name__":                      prometheus.EntityInfoMetricName,
					"job":                           "svc",
					"instance":                      "1",
					prometheus.EntityTypeLabelKey:   "k8s.pod",
					prometheus.EntityIDKeysLabelKey: `["missing_key"]`,
				},
				mtype:   writev2.Metadata_METRIC_TYPE_INFO,
				samples: []writev2.Sample{{Value: 1, Timestamp: 2000}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__": "http_requests",
					"job":      "svc",
					"instance": "1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 1, Timestamp: 2000}},
			},
		)
		md2, _, err := r.translateV2(t.Context(), req2)
		require.NoError(t, err)
		refs2 := entity.ResourceEntityRefs(md2.ResourceMetrics().At(0).Resource())
		require.Equal(t, 1, refs2.Len(), "invalid incoming entity must not evict valid cached entity")
		require.Equal(t, "k8s.pod", refs2.At(0).Type())
		require.Equal(t, []string{"k8s_pod_uid"}, refs2.At(0).IdKeys().AsRaw())

		// Request 3 (metric-only) confirms cache still holds the valid k8s.pod entity.
		req3 := buildPRWV2Request(prwTestSeries{
			labels: map[string]string{
				"__name__": "http_requests",
				"job":      "svc",
				"instance": "1",
			},
			mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
			samples: []writev2.Sample{{Value: 2, Timestamp: 3000}},
		})
		md3, _, err := r.translateV2(t.Context(), req3)
		require.NoError(t, err)
		refs3 := entity.ResourceEntityRefs(md3.ResourceMetrics().At(0).Resource())
		require.Equal(t, 1, refs3.Len())
		require.Equal(t, []string{"k8s_pod_uid"}, refs3.At(0).IdKeys().AsRaw())
	})

	t.Run("same_request_invalid_does_not_overwrite_valid_without_target_info", func(t *testing.T) {
		r := setupMetricsReceiver(t)

		// No target_info yet; service.name comes from job label via parseJobAndInstance.
		// A later invalid same-type entry (missing_key) in the same request must not overwrite
		// the valid entry (service.name).
		req := buildPRWV2Request(
			prwTestSeries{
				labels: map[string]string{
					"__name__":                      prometheus.EntityInfoMetricName,
					"job":                           "svc",
					"instance":                      "1",
					prometheus.EntityTypeLabelKey:   "service",
					prometheus.EntityIDKeysLabelKey: `["service.name"]`,
				},
				mtype:   writev2.Metadata_METRIC_TYPE_INFO,
				samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__":                      prometheus.EntityInfoMetricName,
					"job":                           "svc",
					"instance":                      "1",
					prometheus.EntityTypeLabelKey:   "service",
					prometheus.EntityIDKeysLabelKey: `["missing_key"]`,
				},
				mtype:   writev2.Metadata_METRIC_TYPE_INFO,
				samples: []writev2.Sample{{Value: 1, Timestamp: 2000}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__": "http_requests",
					"job":      "svc",
					"instance": "1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 1, Timestamp: 2000}},
			},
		)
		md, _, err := r.translateV2(t.Context(), req)
		require.NoError(t, err)
		refs := entity.ResourceEntityRefs(md.ResourceMetrics().At(0).Resource())
		require.Equal(t, 1, refs.Len())
		require.Equal(t, "service", refs.At(0).Type())
		require.Equal(t, []string{"service.name"}, refs.At(0).IdKeys().AsRaw())

		// Follow-up metric-only request confirms cache kept the valid entry, not the invalid one.
		followUp := buildPRWV2Request(prwTestSeries{
			labels: map[string]string{
				"__name__": "http_requests",
				"job":      "svc",
				"instance": "1",
			},
			mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
			samples: []writev2.Sample{{Value: 2, Timestamp: 3000}},
		})
		md2, _, err := r.translateV2(t.Context(), followUp)
		require.NoError(t, err)
		refs2 := entity.ResourceEntityRefs(md2.ResourceMetrics().At(0).Resource())
		require.Equal(t, 1, refs2.Len())
		require.Equal(t, []string{"service.name"}, refs2.At(0).IdKeys().AsRaw())
	})

	t.Run("multi_sample_older_stale_and_empty_label_eviction", func(t *testing.T) {
		r := setupMetricsReceiver(t)

		// Request 1: target_info + multi-sample entity where StaleNaN@1000 is followed by active@2000,
		// with an empty-valued optional label otel_entity_schema_url="".
		req1 := buildPRWV2Request(
			prwTestSeries{
				labels: map[string]string{
					"__name__":    "target_info",
					"job":         "svc",
					"instance":    "1",
					"k8s_pod_uid": "pod-1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 1, Timestamp: 2000}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__":                         prometheus.EntityInfoMetricName,
					"job":                              "svc",
					"instance":                         "1",
					prometheus.EntityTypeLabelKey:      "k8s.pod",
					prometheus.EntitySchemaURLLabelKey: "",
					prometheus.EntityIDKeysLabelKey:    `["k8s_pod_uid"]`,
				},
				mtype: writev2.Metadata_METRIC_TYPE_INFO,
				samples: []writev2.Sample{
					{Value: math.Float64frombits(value.StaleNaN), Timestamp: 1000},
					{Value: 1, Timestamp: 2000},
				},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__": "http_requests",
					"job":      "svc",
					"instance": "1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 1, Timestamp: 2000}},
			},
		)
		md1, stats1, err := r.translateV2(t.Context(), req1)
		require.NoError(t, err)
		require.Equal(t, 4, stats1.Samples)
		require.Equal(t, 1, entity.ResourceEntityRefs(md1.ResourceMetrics().At(0).Resource()).Len())

		// Request 2: older StaleNaN@1500 must NOT evict the cached entry@2000.
		req2 := buildPRWV2Request(
			prwTestSeries{
				labels: map[string]string{
					"__name__":                      prometheus.EntityInfoMetricName,
					"job":                           "svc",
					"instance":                      "1",
					prometheus.EntityTypeLabelKey:   "k8s.pod",
					prometheus.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
				},
				mtype:   writev2.Metadata_METRIC_TYPE_INFO,
				samples: []writev2.Sample{{Value: math.Float64frombits(value.StaleNaN), Timestamp: 1500}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__": "http_requests",
					"job":      "svc",
					"instance": "1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 2, Timestamp: 2500}},
			},
		)
		md2, _, err := r.translateV2(t.Context(), req2)
		require.NoError(t, err)
		require.Equal(t, 1, entity.ResourceEntityRefs(md2.ResourceMetrics().At(0).Resource()).Len())

		// Request 3: multi-sample [1@2500, StaleNaN@3000] omitting empty otel_entity_schema_url
		// must normalize empty labels and evict the cached entry@2000.
		req3 := buildPRWV2Request(
			prwTestSeries{
				labels: map[string]string{
					"__name__":                      prometheus.EntityInfoMetricName,
					"job":                           "svc",
					"instance":                      "1",
					prometheus.EntityTypeLabelKey:   "k8s.pod",
					prometheus.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
				},
				mtype: writev2.Metadata_METRIC_TYPE_INFO,
				samples: []writev2.Sample{
					{Value: 1, Timestamp: 2500},
					{Value: math.Float64frombits(value.StaleNaN), Timestamp: 3000},
				},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__": "http_requests",
					"job":      "svc",
					"instance": "1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 3, Timestamp: 3000}},
			},
		)
		md3, _, err := r.translateV2(t.Context(), req3)
		require.NoError(t, err)
		require.Equal(t, 0, entity.ResourceEntityRefs(md3.ResourceMetrics().At(0).Resource()).Len())

		// Request 4: separate active@4000 and StaleNaN@5000 series with the same labels in one request
		// must cancel out (active series suppressed by same-request StaleNaN).
		req4 := buildPRWV2Request(
			prwTestSeries{
				labels: map[string]string{
					"__name__":                      prometheus.EntityInfoMetricName,
					"job":                           "svc",
					"instance":                      "1",
					prometheus.EntityTypeLabelKey:   "k8s.pod",
					prometheus.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
				},
				mtype:   writev2.Metadata_METRIC_TYPE_INFO,
				samples: []writev2.Sample{{Value: 1, Timestamp: 4000}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__":                      prometheus.EntityInfoMetricName,
					"job":                           "svc",
					"instance":                      "1",
					prometheus.EntityTypeLabelKey:   "k8s.pod",
					prometheus.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
				},
				mtype:   writev2.Metadata_METRIC_TYPE_INFO,
				samples: []writev2.Sample{{Value: math.Float64frombits(value.StaleNaN), Timestamp: 5000}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__": "http_requests",
					"job":      "svc",
					"instance": "1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 4, Timestamp: 5000}},
			},
		)
		md4, _, err := r.translateV2(t.Context(), req4)
		require.NoError(t, err)
		require.Equal(t, 0, entity.ResourceEntityRefs(md4.ResourceMetrics().At(0).Resource()).Len())
	})

	t.Run("superseded_same_type_entry_not_resurrected_on_stale_eviction", func(t *testing.T) {
		r := setupMetricsReceiver(t)

		// Request 1 batches S1 (schema v1 @ 1000) and S2 (schema v2 @ 2000) for k8s.pod.
		req1 := buildPRWV2Request(
			prwTestSeries{
				labels: map[string]string{
					"__name__":    "target_info",
					"job":         "svc",
					"instance":    "1",
					"k8s_pod_uid": "pod-1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 1, Timestamp: 2000}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__":                         prometheus.EntityInfoMetricName,
					"job":                              "svc",
					"instance":                         "1",
					prometheus.EntityTypeLabelKey:      "k8s.pod",
					prometheus.EntitySchemaURLLabelKey: "https://opentelemetry.io/schemas/1.30.0",
					prometheus.EntityIDKeysLabelKey:    `["k8s_pod_uid"]`,
				},
				mtype:   writev2.Metadata_METRIC_TYPE_INFO,
				samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__":                         prometheus.EntityInfoMetricName,
					"job":                              "svc",
					"instance":                         "1",
					prometheus.EntityTypeLabelKey:      "k8s.pod",
					prometheus.EntitySchemaURLLabelKey: "https://opentelemetry.io/schemas/1.31.0",
					prometheus.EntityIDKeysLabelKey:    `["k8s_pod_uid"]`,
				},
				mtype:   writev2.Metadata_METRIC_TYPE_INFO,
				samples: []writev2.Sample{{Value: 1, Timestamp: 2000}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__": "http_requests",
					"job":      "svc",
					"instance": "1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 1, Timestamp: 2000}},
			},
		)
		md1, _, err := r.translateV2(t.Context(), req1)
		require.NoError(t, err)
		refs1 := entity.ResourceEntityRefs(md1.ResourceMetrics().At(0).Resource())
		require.Equal(t, 1, refs1.Len())
		require.Equal(t, "https://opentelemetry.io/schemas/1.31.0", refs1.At(0).SchemaUrl())

		// Request 2 evicts S2 via StaleNaN@3000; superseded S1 must NOT be resurrected.
		req2 := buildPRWV2Request(
			prwTestSeries{
				labels: map[string]string{
					"__name__":                         prometheus.EntityInfoMetricName,
					"job":                              "svc",
					"instance":                         "1",
					prometheus.EntityTypeLabelKey:      "k8s.pod",
					prometheus.EntitySchemaURLLabelKey: "https://opentelemetry.io/schemas/1.31.0",
					prometheus.EntityIDKeysLabelKey:    `["k8s_pod_uid"]`,
				},
				mtype:   writev2.Metadata_METRIC_TYPE_INFO,
				samples: []writev2.Sample{{Value: math.Float64frombits(value.StaleNaN), Timestamp: 3000}},
			},
			prwTestSeries{
				labels: map[string]string{
					"__name__": "http_requests",
					"job":      "svc",
					"instance": "1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 2, Timestamp: 3000}},
			},
		)
		md2, _, err := r.translateV2(t.Context(), req2)
		require.NoError(t, err)
		require.Equal(t, 0, entity.ResourceEntityRefs(md2.ResourceMetrics().At(0).Resource()).Len())
	})

	t.Run("concurrent_target_info_and_entity_info_do_not_clobber_cache", func(t *testing.T) {
		r := setupMetricsReceiver(t)

		targetReq := buildPRWV2Request(prwTestSeries{
			labels: map[string]string{
				"__name__":    "target_info",
				"job":         "svc",
				"instance":    "1",
				"k8s_pod_uid": "pod-1",
				"host_id":     "host-1",
			},
			mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
			samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
		})
		podReq := buildPRWV2Request(prwTestSeries{
			labels: map[string]string{
				"__name__":                      prometheus.EntityInfoMetricName,
				"job":                           "svc",
				"instance":                      "1",
				prometheus.EntityTypeLabelKey:   "k8s.pod",
				prometheus.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
			},
			mtype:   writev2.Metadata_METRIC_TYPE_INFO,
			samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
		})
		hostReq := buildPRWV2Request(prwTestSeries{
			labels: map[string]string{
				"__name__":                      prometheus.EntityInfoMetricName,
				"job":                           "svc",
				"instance":                      "1",
				prometheus.EntityTypeLabelKey:   "host",
				prometheus.EntityIDKeysLabelKey: `["host_id"]`,
			},
			mtype:   writev2.Metadata_METRIC_TYPE_INFO,
			samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
		})

		// Run multiple concurrent iterations to exercise overlapping translate + commit paths.
		for range 10 {
			r.rmCache.Purge()
			var wg sync.WaitGroup
			start := make(chan struct{})
			for _, req := range []*writev2.Request{targetReq, podReq, hostReq} {
				wg.Add(1)
				go func(rq *writev2.Request) {
					defer wg.Done()
					<-start
					rec := postPRWV2(t, r, rq)
					require.Equal(t, http.StatusNoContent, rec.Code)
				}(req)
			}
			close(start)
			wg.Wait()

			// Follow-up metric request must observe both target_info attributes and both entities.
			followUp := buildPRWV2Request(prwTestSeries{
				labels: map[string]string{
					"__name__": "http_requests",
					"job":      "svc",
					"instance": "1",
				},
				mtype:   writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 1, Timestamp: 2000}},
			})
			md, _, err := r.translateV2(t.Context(), followUp)
			require.NoError(t, err)
			require.Equal(t, 1, md.ResourceMetrics().Len())
			res := md.ResourceMetrics().At(0).Resource()
			podUID, hasPod := res.Attributes().Get("k8s_pod_uid")
			require.True(t, hasPod)
			require.Equal(t, "pod-1", podUID.Str())
			hostID, hasHost := res.Attributes().Get("host_id")
			require.True(t, hasHost)
			require.Equal(t, "host-1", hostID.Str())
			require.Equal(t, 2, entity.ResourceEntityRefs(res).Len())
		}
	})
}
