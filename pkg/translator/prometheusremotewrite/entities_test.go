// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/xpdata/entity"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

func buildPRWEntityTestMetrics(ts time.Time) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	res := rm.Resource()
	res.Attributes().PutStr("service.namespace", "shop")
	res.Attributes().PutStr("service.name", "checkout")
	res.Attributes().PutStr("service.instance.id", "id-1")
	res.Attributes().PutStr("service.version", "2.4.0")
	res.Attributes().PutStr("k8s.pod.uid", "pod-uid-123")
	res.Attributes().PutStr("k8s.pod.name", "checkout-7f9")

	refs := entity.ResourceEntityRefs(res)
	svc := refs.AppendEmpty()
	svc.SetType("service.instance")
	svc.SetSchemaUrl("https://opentelemetry.io/schemas/1.31.0")
	svc.IdKeys().FromRaw([]string{"service.namespace", "service.name", "service.instance.id"})
	svc.DescriptionKeys().FromRaw([]string{"service.version", "absent.attr"})

	pod := refs.AppendEmpty()
	pod.SetType("k8s.pod")
	pod.IdKeys().FromRaw([]string{"k8s.pod.uid"})
	pod.DescriptionKeys().FromRaw([]string{"k8s.pod.name"})

	// Missing id key -> must be skipped.
	invalid := refs.AppendEmpty()
	invalid.SetType("host")
	invalid.IdKeys().FromRaw([]string{"host.id"})

	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("http_requests")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(10)
	if !ts.IsZero() {
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	}
	return md
}

func v1LabelsToMap(lbls []prompb.Label) map[string]string {
	out := make(map[string]string, len(lbls))
	for _, l := range lbls {
		out[l.Name] = l.Value
	}
	return out
}

func v2LabelsToMap(ts *writev2.TimeSeries, syms []string) map[string]string {
	out := make(map[string]string, len(ts.LabelsRefs)/2)
	for i := 0; i+1 < len(ts.LabelsRefs); i += 2 {
		out[syms[ts.LabelsRefs[i]]] = syms[ts.LabelsRefs[i+1]]
	}
	return out
}

func TestFromMetricsEntitiesV1AndV2(t *testing.T) {
	ts := time.Unix(1700000000, 0)
	tsMs := convertTimeStamp(pcommon.NewTimestampFromTime(ts))

	tests := []struct {
		name     string
		strategy string
		want     map[string]map[string]string
	}{
		{
			name:     "underscore_escaping",
			strategy: "UnderscoreEscapingWithSuffixes",
			want: map[string]map[string]string{
				"service.instance": {
					model.MetricNameLabel:                              prometheustranslator.EntityInfoMetricName,
					model.JobLabel:                                     "shop/checkout",
					model.InstanceLabel:                                "id-1",
					prometheustranslator.EntityTypeLabelKey:            "service.instance",
					prometheustranslator.EntitySchemaURLLabelKey:       "https://opentelemetry.io/schemas/1.31.0",
					prometheustranslator.EntityIDKeysLabelKey:          `["service_instance_id","service_name","service_namespace"]`,
					prometheustranslator.EntityDescriptionKeysLabelKey: `["service_version"]`,
				},
				"k8s.pod": {
					model.MetricNameLabel:                              prometheustranslator.EntityInfoMetricName,
					model.JobLabel:                                     "shop/checkout",
					model.InstanceLabel:                                "id-1",
					prometheustranslator.EntityTypeLabelKey:            "k8s.pod",
					prometheustranslator.EntityIDKeysLabelKey:          `["k8s_pod_uid"]`,
					prometheustranslator.EntityDescriptionKeysLabelKey: `["k8s_pod_name"]`,
				},
			},
		},
		{
			name:     "no_translation",
			strategy: "NoTranslation",
			want: map[string]map[string]string{
				"service.instance": {
					model.MetricNameLabel:                              prometheustranslator.EntityInfoMetricName,
					model.JobLabel:                                     "shop/checkout",
					model.InstanceLabel:                                "id-1",
					prometheustranslator.EntityTypeLabelKey:            "service.instance",
					prometheustranslator.EntitySchemaURLLabelKey:       "https://opentelemetry.io/schemas/1.31.0",
					prometheustranslator.EntityIDKeysLabelKey:          `["service.instance.id","service.name","service.namespace"]`,
					prometheustranslator.EntityDescriptionKeysLabelKey: `["service.version"]`,
				},
				"k8s.pod": {
					model.MetricNameLabel:                              prometheustranslator.EntityInfoMetricName,
					model.JobLabel:                                     "shop/checkout",
					model.InstanceLabel:                                "id-1",
					prometheustranslator.EntityTypeLabelKey:            "k8s.pod",
					prometheustranslator.EntityIDKeysLabelKey:          `["k8s.pod.uid"]`,
					prometheustranslator.EntityDescriptionKeysLabelKey: `["k8s.pod.name"]`,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := buildPRWEntityTestMetrics(ts)
			settings := Settings{
				Namespace:           "custom_ns",
				ExternalLabels:      map[string]string{"env": "prod"},
				TranslationStrategy: tt.strategy,
			}

			// PRW v1
			tsMapV1, err := FromMetrics(md, settings)
			require.NoError(t, err)
			gotV1 := map[string]map[string]string{}
			for _, series := range tsMapV1 {
				lbls := v1LabelsToMap(series.Labels)
				if lbls[model.MetricNameLabel] == prometheustranslator.EntityInfoMetricName {
					require.Len(t, series.Samples, 1)
					require.Equal(t, float64(1), series.Samples[0].Value)
					require.Equal(t, tsMs, series.Samples[0].Timestamp)
					gotV1[lbls[prometheustranslator.EntityTypeLabelKey]] = lbls
				}
			}
			require.Equal(t, tt.want, gotV1)

			// PRW v2
			tsMapV2, symTable, err := FromMetricsV2(md, settings)
			require.NoError(t, err)
			syms := symTable.Symbols()
			gotV2 := map[string]map[string]string{}
			for _, series := range tsMapV2 {
				lbls := v2LabelsToMap(series, syms)
				if lbls[model.MetricNameLabel] == prometheustranslator.EntityInfoMetricName {
					require.Len(t, series.Samples, 1)
					require.Equal(t, float64(1), series.Samples[0].Value)
					require.Equal(t, tsMs, series.Samples[0].Timestamp)
					require.Equal(t, writev2.Metadata_METRIC_TYPE_INFO, series.Metadata.Type)
					require.Equal(t, "Entity metadata", syms[series.Metadata.HelpRef])
					gotV2[lbls[prometheustranslator.EntityTypeLabelKey]] = lbls
				}
			}
			require.Equal(t, tt.want, gotV2)
		})
	}
}

func TestFromMetricsEntitiesGatingAndDeduplication(t *testing.T) {
	t.Run("disable_target_info_skips_entities", func(t *testing.T) {
		md := buildPRWEntityTestMetrics(time.Unix(1700000000, 0))
		settings := Settings{DisableTargetInfo: true}

		v1, err := FromMetrics(md, settings)
		require.NoError(t, err)
		for _, s := range v1 {
			require.NotEqual(t, prometheustranslator.EntityInfoMetricName, v1LabelsToMap(s.Labels)[model.MetricNameLabel])
		}

		v2, syms, err := FromMetricsV2(md, settings)
		require.NoError(t, err)
		for _, s := range v2 {
			require.NotEqual(t, prometheustranslator.EntityInfoMetricName, v2LabelsToMap(s, syms.Symbols())[model.MetricNameLabel])
		}
	})

	t.Run("zero_timestamp_skips_entities", func(t *testing.T) {
		md := buildPRWEntityTestMetrics(time.Time{})
		v1, err := FromMetrics(md, Settings{})
		require.NoError(t, err)
		for _, s := range v1 {
			require.NotEqual(t, prometheustranslator.EntityInfoMetricName, v1LabelsToMap(s.Labels)[model.MetricNameLabel])
		}

		v2, syms, err := FromMetricsV2(md, Settings{})
		require.NoError(t, err)
		for _, s := range v2 {
			require.NotEqual(t, prometheustranslator.EntityInfoMetricName, v2LabelsToMap(s, syms.Symbols())[model.MetricNameLabel])
		}
	})

	t.Run("external_job_label_copied_without_other_external_labels", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		res := rm.Resource()
		res.Attributes().PutStr("k8s.pod.uid", "pod-1")
		er := entity.ResourceEntityRefs(res).AppendEmpty()
		er.SetType("k8s.pod")
		er.IdKeys().FromRaw([]string{"k8s.pod.uid"})

		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("cpu_usage")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetDoubleValue(1)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1700000000, 0)))

		settings := Settings{
			ExternalLabels: map[string]string{
				"job":    "ext-job",
				"region": "us-east",
			},
		}
		v2, syms, err := FromMetricsV2(md, settings)
		require.NoError(t, err)
		var found map[string]string
		for _, s := range v2 {
			lbls := v2LabelsToMap(s, syms.Symbols())
			if lbls[model.MetricNameLabel] == prometheustranslator.EntityInfoMetricName {
				found = lbls
			}
		}
		require.Equal(t, map[string]string{
			model.MetricNameLabel:                     prometheustranslator.EntityInfoMetricName,
			model.JobLabel:                            "ext-job",
			prometheustranslator.EntityTypeLabelKey:   "k8s.pod",
			prometheustranslator.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
		}, found)
	})

	t.Run("duplicate_job_instance_keeps_latest_timestamp_resource", func(t *testing.T) {
		for _, newerFirst := range []bool{false, true} {
			name := "older_then_newer"
			if newerFirst {
				name = "newer_then_older"
			}
			t.Run(name, func(t *testing.T) {
				md := pmetric.NewMetrics()

				addOlder := func() {
					rm1 := md.ResourceMetrics().AppendEmpty()
					rm1.Resource().Attributes().PutStr("service.name", "api")
					rm1.Resource().Attributes().PutStr("service.instance.id", "1")
					rm1.Resource().Attributes().PutStr("k8s.pod.uid", "pod-1")
					rm1.Resource().Attributes().PutStr("k8s.pod.name", "old-pod")
					e1 := entity.ResourceEntityRefs(rm1.Resource()).AppendEmpty()
					e1.SetType("k8s.pod")
					e1.IdKeys().FromRaw([]string{"k8s.pod.uid"})
					e1.DescriptionKeys().FromRaw([]string{"k8s.pod.name"})
					m1 := rm1.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
					m1.SetName("requests")
					dp1 := m1.SetEmptyGauge().DataPoints().AppendEmpty()
					dp1.SetDoubleValue(1)
					dp1.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(100, 0)))
				}
				addNewer := func() {
					rm2 := md.ResourceMetrics().AppendEmpty()
					rm2.Resource().Attributes().PutStr("service.name", "api")
					rm2.Resource().Attributes().PutStr("service.instance.id", "1")
					rm2.Resource().Attributes().PutStr("k8s.pod.uid", "pod-1")
					e2 := entity.ResourceEntityRefs(rm2.Resource()).AppendEmpty()
					e2.SetType("k8s.pod")
					e2.IdKeys().FromRaw([]string{"k8s.pod.uid"})
					m2 := rm2.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
					m2.SetName("requests")
					dp2 := m2.SetEmptyGauge().DataPoints().AppendEmpty()
					dp2.SetDoubleValue(2)
					dp2.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(200, 0)))
				}
				if newerFirst {
					addNewer()
					addOlder()
				} else {
					addOlder()
					addNewer()
				}

				v1, err := FromMetrics(md, Settings{})
				require.NoError(t, err)
				var v1EntitySeries []*prompb.TimeSeries
				for _, s := range v1 {
					if v1LabelsToMap(s.Labels)[model.MetricNameLabel] == prometheustranslator.EntityInfoMetricName {
						v1EntitySeries = append(v1EntitySeries, s)
					}
				}
				require.Len(t, v1EntitySeries, 1)
				require.Equal(t, map[string]string{
					model.MetricNameLabel:                     prometheustranslator.EntityInfoMetricName,
					model.JobLabel:                            "api",
					model.InstanceLabel:                       "1",
					prometheustranslator.EntityTypeLabelKey:   "k8s.pod",
					prometheustranslator.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
				}, v1LabelsToMap(v1EntitySeries[0].Labels))
				require.Equal(t, int64(200_000), v1EntitySeries[0].Samples[0].Timestamp)

				v2, syms, err := FromMetricsV2(md, Settings{})
				require.NoError(t, err)
				var v2EntitySeries []*writev2.TimeSeries
				for _, s := range v2 {
					if v2LabelsToMap(s, syms.Symbols())[model.MetricNameLabel] == prometheustranslator.EntityInfoMetricName {
						v2EntitySeries = append(v2EntitySeries, s)
					}
				}
				require.Len(t, v2EntitySeries, 1)
				require.Equal(t, map[string]string{
					model.MetricNameLabel:                     prometheustranslator.EntityInfoMetricName,
					model.JobLabel:                            "api",
					model.InstanceLabel:                       "1",
					prometheustranslator.EntityTypeLabelKey:   "k8s.pod",
					prometheustranslator.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
				}, v2LabelsToMap(v2EntitySeries[0], syms.Symbols()))
				require.Equal(t, int64(200_000), v2EntitySeries[0].Samples[0].Timestamp)
			})
		}
	})

	t.Run("empty_valued_resource_attributes_omitted_from_entity_info", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		res := rm.Resource()
		res.Attributes().PutStr("service.name", "api")
		res.Attributes().PutStr("service.instance.id", "1")
		res.Attributes().PutStr("k8s.pod.uid", "pod-1")
		res.Attributes().PutStr("k8s.pod.name", "")
		res.Attributes().PutStr("host.id", "")

		refs := entity.ResourceEntityRefs(res)
		pod := refs.AppendEmpty()
		pod.SetType("k8s.pod")
		pod.IdKeys().FromRaw([]string{"k8s.pod.uid"})
		pod.DescriptionKeys().FromRaw([]string{"k8s.pod.name"})

		host := refs.AppendEmpty()
		host.SetType("host")
		host.IdKeys().FromRaw([]string{"host.id"})

		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("requests")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetDoubleValue(1)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(100, 0)))

		v2, syms, err := FromMetricsV2(md, Settings{})
		require.NoError(t, err)
		var entitySeries []map[string]string
		for _, s := range v2 {
			lbls := v2LabelsToMap(s, syms.Symbols())
			if lbls[model.MetricNameLabel] == prometheustranslator.EntityInfoMetricName {
				entitySeries = append(entitySeries, lbls)
			}
		}
		require.Len(t, entitySeries, 1)
		require.Equal(t, map[string]string{
			model.MetricNameLabel:                     prometheustranslator.EntityInfoMetricName,
			model.JobLabel:                            "api",
			model.InstanceLabel:                       "1",
			prometheustranslator.EntityTypeLabelKey:   "k8s.pod",
			prometheustranslator.EntityIDKeysLabelKey: `["k8s_pod_uid"]`,
		}, entitySeries[0])
	})
}
