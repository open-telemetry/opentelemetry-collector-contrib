// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"math"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/xpdata/entity"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

func TestTransactionEntityInfo(t *testing.T) {
	t.Run("otel_entity_info before and after target_info populates Resource.entity_refs", func(t *testing.T) {
		for _, entityFirst := range []bool{true, false} {
			name := "target_first"
			if entityFirst {
				name = "entity_first"
			}
			t.Run(name, func(t *testing.T) {
				mdStore := testMetadataStore(map[string]scrape.MetricMetadata{
					"otel_entity": {MetricFamily: "otel_entity", Type: model.MetricTypeInfo},
					"target":      {MetricFamily: "target", Type: model.MetricTypeInfo},
					"test_gauge":  {MetricFamily: "test_gauge", Type: model.MetricTypeGauge},
				})
				ctx := scrape.ContextWithMetricMetadataStore(
					scrape.ContextWithTarget(context.Background(), target),
					mdStore,
				)
				sink := new(consumertest.MetricsSink)
				tr := newTransaction(ctx, sink, labels.EmptyLabels(), receivertest.NewNopSettings(receivertest.NopType), nopObsRecv(t), false, true)

				entitySvc := labels.FromStrings(
					model.MetricNameLabel, prometheus.EntityInfoMetricName,
					model.JobLabel, "shop/checkout",
					model.InstanceLabel, "id-1",
					prometheus.EntityTypeLabelKey, "service.instance",
					prometheus.EntitySchemaURLLabelKey, "https://opentelemetry.io/schemas/1.31.0",
					prometheus.EntityIDKeysLabelKey, `["service_instance_id","service_name","service_namespace"]`,
					prometheus.EntityDescriptionKeysLabelKey, `["service_version"]`,
					"otel_entity_future_field", "ignored",
				)
				entityPod := labels.FromStrings(
					model.MetricNameLabel, prometheus.EntityInfoMetricName,
					model.JobLabel, "shop/checkout",
					model.InstanceLabel, "id-1",
					prometheus.EntityTypeLabelKey, "k8s.pod",
					prometheus.EntityIDKeysLabelKey, `["k8s_pod_uid"]`,
					prometheus.EntityDescriptionKeysLabelKey, `["k8s_pod_name","missing_attr"]`,
				)
				targetInfo := labels.FromStrings(
					model.MetricNameLabel, prometheus.TargetInfoMetricName,
					model.JobLabel, "shop/checkout",
					model.InstanceLabel, "id-1",
					"service_namespace", "shop",
					"service_name", "checkout",
					"service_instance_id", "id-1",
					"service_version", "2.4.0",
					"k8s_pod_uid", "pod-uid-123",
					"k8s_pod_name", "checkout-7f9",
				)
				sample := labels.FromStrings(
					model.MetricNameLabel, "test_gauge",
					model.JobLabel, "shop/checkout",
					model.InstanceLabel, "id-1",
				)

				if entityFirst {
					_, err := tr.Append(0, entitySvc, 0, ts, 1, nil, nil, storage.AOptions{})
					require.NoError(t, err)
					_, err = tr.Append(0, entityPod, 0, ts, 1, nil, nil, storage.AOptions{})
					require.NoError(t, err)
					_, err = tr.Append(0, targetInfo, 0, ts, 1, nil, nil, storage.AOptions{})
					require.NoError(t, err)
				} else {
					_, err := tr.Append(0, targetInfo, 0, ts, 1, nil, nil, storage.AOptions{})
					require.NoError(t, err)
					_, err = tr.Append(0, entitySvc, 0, ts, 1, nil, nil, storage.AOptions{})
					require.NoError(t, err)
					_, err = tr.Append(0, entityPod, 0, ts, 1, nil, nil, storage.AOptions{})
					require.NoError(t, err)
				}
				_, err := tr.Append(0, sample, 0, ts, 42, nil, nil, storage.AOptions{})
				require.NoError(t, err)
				require.NoError(t, tr.Commit())

				all := sink.AllMetrics()
				require.Len(t, all, 1)
				require.Equal(t, 1, all[0].ResourceMetrics().Len())
				rm := all[0].ResourceMetrics().At(0)
				// otel_entity_info must be dropped from metrics.
				require.Equal(t, 1, rm.ScopeMetrics().Len())
				require.Equal(t, 1, rm.ScopeMetrics().At(0).Metrics().Len())
				assert.Equal(t, "test_gauge", rm.ScopeMetrics().At(0).Metrics().At(0).Name())

				refs := entity.ResourceEntityRefs(rm.Resource())
				require.Equal(t, 2, refs.Len())
				assert.Equal(t, "service.instance", refs.At(0).Type())
				assert.Equal(t, "https://opentelemetry.io/schemas/1.31.0", refs.At(0).SchemaUrl())
				assert.Equal(t, []string{"service_instance_id", "service_name", "service_namespace"}, refs.At(0).IdKeys().AsRaw())
				assert.Equal(t, []string{"service_version"}, refs.At(0).DescriptionKeys().AsRaw())

				assert.Equal(t, "k8s.pod", refs.At(1).Type())
				assert.Empty(t, refs.At(1).SchemaUrl())
				assert.Equal(t, []string{"k8s_pod_uid"}, refs.At(1).IdKeys().AsRaw())
				assert.Equal(t, []string{"k8s_pod_name"}, refs.At(1).DescriptionKeys().AsRaw())
			})
		}
	})

	t.Run("ignores StaleNaN, invalid entities, and does not create resource alone", func(t *testing.T) {
		mdStore := testMetadataStore(map[string]scrape.MetricMetadata{
			"otel_entity_info": {MetricFamily: "otel_entity_info", Type: model.MetricTypeGauge},
			"test_gauge":       {MetricFamily: "test_gauge", Type: model.MetricTypeGauge},
		})
		ctx := scrape.ContextWithMetricMetadataStore(
			scrape.ContextWithTarget(context.Background(), target),
			mdStore,
		)
		sink := new(consumertest.MetricsSink)
		tr := newTransaction(ctx, sink, labels.EmptyLabels(), receivertest.NewNopSettings(receivertest.NopType), nopObsRecv(t), false, true)

		// Target A: only otel_entity_info, no metrics -> must not produce a ResourceMetrics.
		onlyEntity := labels.FromStrings(
			model.MetricNameLabel, prometheus.EntityInfoMetricName,
			model.JobLabel, "only-entity",
			model.InstanceLabel, "inst-a",
			prometheus.EntityTypeLabelKey, "host",
			prometheus.EntityIDKeysLabelKey, `["service.name"]`,
		)
		_, err := tr.Append(0, onlyEntity, 0, ts, 1, nil, nil, storage.AOptions{})
		require.NoError(t, err)

		// Target B: StaleNaN entity + malformed entity + missing id_key entity + valid metric.
		staleEntity := labels.FromStrings(
			model.MetricNameLabel, prometheus.EntityInfoMetricName,
			model.JobLabel, "job-b",
			model.InstanceLabel, "inst-b",
			prometheus.EntityTypeLabelKey, "service",
			prometheus.EntityIDKeysLabelKey, `["service.name"]`,
		)
		_, err = tr.Append(0, staleEntity, 0, ts, math.Float64frombits(value.StaleNaN), nil, nil, storage.AOptions{})
		require.NoError(t, err)

		malformedEntity := labels.FromStrings(
			model.MetricNameLabel, prometheus.EntityInfoMetricName,
			model.JobLabel, "job-b",
			model.InstanceLabel, "inst-b",
			prometheus.EntityTypeLabelKey, "service",
			prometheus.EntityIDKeysLabelKey, "not-valid-json",
		)
		_, err = tr.Append(0, malformedEntity, 0, ts, 1, nil, nil, storage.AOptions{})
		require.NoError(t, err)

		missingAttrEntity := labels.FromStrings(
			model.MetricNameLabel, prometheus.EntityInfoMetricName,
			model.JobLabel, "job-b",
			model.InstanceLabel, "inst-b",
			prometheus.EntityTypeLabelKey, "k8s.pod",
			prometheus.EntityIDKeysLabelKey, `["k8s_pod_uid"]`,
		)
		_, err = tr.Append(0, missingAttrEntity, 0, ts, 1, nil, nil, storage.AOptions{})
		require.NoError(t, err)

		sampleB := labels.FromStrings(
			model.MetricNameLabel, "test_gauge",
			model.JobLabel, "job-b",
			model.InstanceLabel, "inst-b",
		)
		_, err = tr.Append(0, sampleB, 0, ts, 1, nil, nil, storage.AOptions{})
		require.NoError(t, err)
		require.NoError(t, tr.Commit())

		all := sink.AllMetrics()
		require.Len(t, all, 1)
		require.Equal(t, 1, all[0].ResourceMetrics().Len())
		rm := all[0].ResourceMetrics().At(0)
		svcName, _ := rm.Resource().Attributes().Get("service.name")
		assert.Equal(t, "job-b", svcName.Str())
		assert.Equal(t, 0, entity.ResourceEntityRefs(rm.Resource()).Len())
	})

	t.Run("counter-typed otel_entity_info is not treated as entity metadata", func(t *testing.T) {
		mdStore := testMetadataStore(map[string]scrape.MetricMetadata{
			"otel_entity_info": {MetricFamily: "otel_entity_info", Type: model.MetricTypeCounter},
		})
		ctx := scrape.ContextWithMetricMetadataStore(
			scrape.ContextWithTarget(context.Background(), target),
			mdStore,
		)
		sink := new(consumertest.MetricsSink)
		tr := newTransaction(ctx, sink, labels.EmptyLabels(), receivertest.NewNopSettings(receivertest.NopType), nopObsRecv(t), false, true)

		counterMetric := labels.FromStrings(
			model.MetricNameLabel, prometheus.EntityInfoMetricName,
			model.JobLabel, "job-c",
			model.InstanceLabel, "inst-c",
			prometheus.EntityTypeLabelKey, "service",
			prometheus.EntityIDKeysLabelKey, `["service.name"]`,
		)
		_, err := tr.Append(0, counterMetric, 0, ts, 5, nil, nil, storage.AOptions{})
		require.NoError(t, err)
		require.NoError(t, tr.Commit())

		all := sink.AllMetrics()
		require.Len(t, all, 1)
		require.Equal(t, 1, all[0].ResourceMetrics().Len())
		rm := all[0].ResourceMetrics().At(0)
		assert.Equal(t, 0, entity.ResourceEntityRefs(rm.Resource()).Len())
		require.Equal(t, 1, rm.ScopeMetrics().At(0).Metrics().Len())
		assert.Equal(t, prometheus.EntityInfoMetricName, rm.ScopeMetrics().At(0).Metrics().At(0).Name())
	})

	t.Run("reconciles out-of-order same-type otel_entity_info by timestamp", func(t *testing.T) {
		mdStore := testMetadataStore(map[string]scrape.MetricMetadata{
			"otel_entity": {MetricFamily: "otel_entity", Type: model.MetricTypeInfo},
			"target":      {MetricFamily: "target", Type: model.MetricTypeInfo},
			"test_gauge":  {MetricFamily: "test_gauge", Type: model.MetricTypeGauge},
		})
		ctx := scrape.ContextWithMetricMetadataStore(
			scrape.ContextWithTarget(context.Background(), target),
			mdStore,
		)
		sink := new(consumertest.MetricsSink)
		tr := newTransaction(ctx, sink, labels.EmptyLabels(), receivertest.NewNopSettings(receivertest.NopType), nopObsRecv(t), false, true)

		targetInfo := labels.FromStrings(
			model.MetricNameLabel, prometheus.TargetInfoMetricName,
			model.JobLabel, "job-d",
			model.InstanceLabel, "inst-d",
			"host_id", "h-1",
			"host_name", "node-1",
			"k8s_pod_uid", "pod-1",
			"k8s_pod_name", "pod-a",
			"k8s_pod_ip", "10.0.0.1",
		)
		_, err := tr.Append(0, targetInfo, 0, ts, 1, nil, nil, storage.AOptions{})
		require.NoError(t, err)

		// Append newer schema_url (ts+100) BEFORE older schema_url (ts): newer timestamp must win.
		hostNewer := labels.FromStrings(
			model.MetricNameLabel, prometheus.EntityInfoMetricName,
			model.JobLabel, "job-d",
			model.InstanceLabel, "inst-d",
			prometheus.EntityTypeLabelKey, "host",
			prometheus.EntitySchemaURLLabelKey, "https://opentelemetry.io/schemas/1.31.0",
			prometheus.EntityIDKeysLabelKey, `["host_id"]`,
		)
		hostOlder := labels.FromStrings(
			model.MetricNameLabel, prometheus.EntityInfoMetricName,
			model.JobLabel, "job-d",
			model.InstanceLabel, "inst-d",
			prometheus.EntityTypeLabelKey, "host",
			prometheus.EntitySchemaURLLabelKey, "https://opentelemetry.io/schemas/1.30.0",
			prometheus.EntityIDKeysLabelKey, `["host_id"]`,
			prometheus.EntityDescriptionKeysLabelKey, `["host_name"]`,
		)
		_, err = tr.Append(0, hostNewer, 0, ts+100, 1, nil, nil, storage.AOptions{})
		require.NoError(t, err)
		_, err = tr.Append(0, hostOlder, 0, ts, 1, nil, nil, storage.AOptions{})
		require.NoError(t, err)

		// Same identity & schema_url with distinct description keys must merge.
		pod1 := labels.FromStrings(
			model.MetricNameLabel, prometheus.EntityInfoMetricName,
			model.JobLabel, "job-d",
			model.InstanceLabel, "inst-d",
			prometheus.EntityTypeLabelKey, "k8s.pod",
			prometheus.EntityIDKeysLabelKey, `["k8s_pod_uid"]`,
			prometheus.EntityDescriptionKeysLabelKey, `["k8s_pod_name"]`,
		)
		pod2 := labels.FromStrings(
			model.MetricNameLabel, prometheus.EntityInfoMetricName,
			model.JobLabel, "job-d",
			model.InstanceLabel, "inst-d",
			prometheus.EntityTypeLabelKey, "k8s.pod",
			prometheus.EntityIDKeysLabelKey, `["k8s_pod_uid"]`,
			prometheus.EntityDescriptionKeysLabelKey, `["k8s_pod_ip"]`,
		)
		_, err = tr.Append(0, pod1, 0, ts+50, 1, nil, nil, storage.AOptions{})
		require.NoError(t, err)
		_, err = tr.Append(0, pod2, 0, ts, 1, nil, nil, storage.AOptions{})
		require.NoError(t, err)

		sample := labels.FromStrings(
			model.MetricNameLabel, "test_gauge",
			model.JobLabel, "job-d",
			model.InstanceLabel, "inst-d",
		)
		_, err = tr.Append(0, sample, 0, ts, 1, nil, nil, storage.AOptions{})
		require.NoError(t, err)
		require.NoError(t, tr.Commit())

		all := sink.AllMetrics()
		require.Len(t, all, 1)
		refs := entity.ResourceEntityRefs(all[0].ResourceMetrics().At(0).Resource())
		require.Equal(t, 2, refs.Len())
		byType := map[string]entity.EntityRef{}
		for i := 0; i < refs.Len(); i++ {
			byType[refs.At(i).Type()] = refs.At(i)
		}

		require.Contains(t, byType, "host")
		assert.Equal(t, "https://opentelemetry.io/schemas/1.31.0", byType["host"].SchemaUrl())
		assert.Empty(t, byType["host"].DescriptionKeys().AsRaw())

		require.Contains(t, byType, "k8s.pod")
		assert.Equal(t, []string{"k8s_pod_uid"}, byType["k8s.pod"].IdKeys().AsRaw())
		assert.Equal(t, []string{"k8s_pod_ip", "k8s_pod_name"}, byType["k8s.pod"].DescriptionKeys().AsRaw())
	})
}
