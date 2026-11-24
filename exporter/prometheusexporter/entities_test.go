// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/xpdata/entity"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

func buildMetricsWithEntities() pmetric.Metrics {
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
	svc.DescriptionKeys().FromRaw([]string{"service.version", "missing.desc"})

	pod := refs.AppendEmpty()
	pod.SetType("k8s.pod")
	pod.IdKeys().FromRaw([]string{"k8s.pod.uid"})
	pod.DescriptionKeys().FromRaw([]string{"k8s.pod.name"})

	// Entity with missing identifying attribute must be omitted.
	invalid := refs.AppendEmpty()
	invalid.SetType("host")
	invalid.IdKeys().FromRaw([]string{"host.id"})

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")
	m := sm.Metrics().AppendEmpty()
	m.SetName("http_requests")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(42)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Second metric under the same (job, instance) to verify deduplication.
	m2 := sm.Metrics().AppendEmpty()
	m2.SetName("http_errors")
	dp2 := m2.SetEmptyGauge().DataPoints().AppendEmpty()
	dp2.SetDoubleValue(1)
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	return md
}

func TestEntityInfoMetricsExport(t *testing.T) {
	tests := []struct {
		name              string
		strategy          translationStrategy
		enableOpenMetrics bool
		wantEntities      map[string]map[string]string
	}{
		{
			name:     "underscore_escaping",
			strategy: underscoreEscapingWithSuffixes,
			wantEntities: map[string]map[string]string{
				"k8s.pod": {
					"job":                          "shop/checkout",
					"instance":                     "id-1",
					"otel_entity_type":             "k8s.pod",
					"otel_entity_id_keys":          `["k8s_pod_uid"]`,
					"otel_entity_description_keys": `["k8s_pod_name"]`,
				},
				"service.instance": {
					"job":                          "shop/checkout",
					"instance":                     "id-1",
					"otel_entity_type":             "service.instance",
					"otel_entity_schema_url":       "https://opentelemetry.io/schemas/1.31.0",
					"otel_entity_id_keys":          `["service_instance_id","service_name","service_namespace"]`,
					"otel_entity_description_keys": `["service_version"]`,
				},
			},
		},
		{
			name:              "no_translation_utf8_openmetrics",
			strategy:          noTranslation,
			enableOpenMetrics: true,
			wantEntities: map[string]map[string]string{
				"k8s.pod": {
					"job":                          "shop/checkout",
					"instance":                     "id-1",
					"otel_entity_type":             "k8s.pod",
					"otel_entity_id_keys":          `["k8s.pod.uid"]`,
					"otel_entity_description_keys": `["k8s.pod.name"]`,
				},
				"service.instance": {
					"job":                          "shop/checkout",
					"instance":                     "id-1",
					"otel_entity_type":             "service.instance",
					"otel_entity_schema_url":       "https://opentelemetry.io/schemas/1.31.0",
					"otel_entity_id_keys":          `["service.instance.id","service.name","service.namespace"]`,
					"otel_entity_description_keys": `["service.version"]`,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Transport: "tcp",
						Endpoint:  "localhost:0",
					},
				},
				ConstLabels:         prometheus.Labels{"extra_const": "should_not_appear"},
				MetricExpiration:    time.Hour,
				EnableOpenMetrics:   tt.enableOpenMetrics,
				TranslationStrategy: tt.strategy,
			}
			set := exportertest.NewNopSettings(metadata.Type)
			exp, err := newPrometheusExporter(cfg, set)
			require.NoError(t, err)

			require.NoError(t, exp.ConsumeMetrics(t.Context(), buildMetricsWithEntities()))

			// Verify via registry Gather that otel_entity_info has the exact expected series.
			mfs, err := exp.registry.Gather()
			require.NoError(t, err)

			var entityMF *dto.MetricFamily
			for _, mf := range mfs {
				if mf.GetName() == prometheustranslator.EntityInfoMetricName {
					entityMF = mf
					break
				}
			}
			require.NotNil(t, entityMF, "expected otel_entity_info metric family")
			require.Equal(t, dto.MetricType_GAUGE, entityMF.GetType())
			require.Len(t, entityMF.GetMetric(), len(tt.wantEntities))

			gotEntities := make(map[string]map[string]string, len(entityMF.GetMetric()))
			for _, m := range entityMF.GetMetric() {
				require.Equal(t, float64(1), m.GetGauge().GetValue())
				lbls := make(map[string]string, len(m.GetLabel()))
				for _, lp := range m.GetLabel() {
					lbls[lp.GetName()] = lp.GetValue()
				}
				gotEntities[lbls[prometheustranslator.EntityTypeLabelKey]] = lbls
			}
			require.Equal(t, tt.wantEntities, gotEntities)

			// Verify HTTP scrape endpoint succeeds without error (including OpenMetrics).
			req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
			if tt.enableOpenMetrics {
				req.Header.Set("Accept", "application/openmetrics-text; version=1.0.0; charset=utf-8")
			}
			rec := httptest.NewRecorder()
			exp.handler.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
			body, err := io.ReadAll(rec.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), prometheustranslator.EntityInfoMetricName)
			require.NotContains(t, string(body), `otel_entity_info{extra_const=`)
		})
	}
}

func TestEntityInfoSkippedWithoutTargetInfo(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	res := rm.Resource()
	// Missing service.instance.id -> no target_info -> no otel_entity_info.
	res.Attributes().PutStr("service.name", "checkout")
	res.Attributes().PutStr("k8s.pod.uid", "pod-uid-123")
	er := entity.ResourceEntityRefs(res).AppendEmpty()
	er.SetType("k8s.pod")
	er.IdKeys().FromRaw([]string{"k8s.pod.uid"})

	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("up_metric")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1)

	c := newCollector(&Config{MetricExpiration: time.Hour}, zap.NewNop())
	c.processMetrics(rm)

	reg := prometheus.NewRegistry()
	require.NoError(t, reg.Register(c))
	mfs, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		require.NotEqual(t, prometheustranslator.TargetInfoMetricName, mf.GetName())
		require.NotEqual(t, prometheustranslator.EntityInfoMetricName, mf.GetName())
	}
}

func TestEndToEndEntitiesRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("This test scrapes over loopback HTTP")
	}

	defer testutil.SetFeatureGateForTest(t, prometheustranslator.KeepIdentifyingResourceAttributesGate, true)()

	for _, tc := range []struct {
		name              string
		strategy          translationStrategy
		enableOpenMetrics bool
		wantSvcIDKeys     []string
		wantSvcDescKeys   []string
		wantPodIDKeys     []string
		wantPodDescKeys   []string
	}{
		{
			name:            "underscore_escaping",
			strategy:        underscoreEscapingWithSuffixes,
			wantSvcIDKeys:   []string{"service_instance_id", "service_name", "service_namespace"},
			wantSvcDescKeys: []string{"service_version"},
			wantPodIDKeys:   []string{"k8s_pod_uid"},
			wantPodDescKeys: []string{"k8s_pod_name"},
		},
		{
			name:              "no_translation_openmetrics",
			strategy:          noTranslation,
			enableOpenMetrics: true,
			wantSvcIDKeys:     []string{"service.instance.id", "service.name", "service.namespace"},
			wantSvcDescKeys:   []string{"service.version"},
			wantPodIDKeys:     []string{"k8s.pod.uid"},
			wantPodDescKeys:   []string{"k8s.pod.name"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			addr := testutil.GetAvailableLocalAddress(t)
			serverConfig := confighttp.NewDefaultServerConfig()
			serverConfig.NetAddr = confignet.AddrConfig{
				Transport: "tcp",
				Endpoint:  addr,
			}
			expCfg := &Config{
				ServerConfig:        serverConfig,
				MetricExpiration:    time.Hour,
				EnableOpenMetrics:   tc.enableOpenMetrics,
				TranslationStrategy: tc.strategy,
			}
			exp, err := NewFactory().CreateMetrics(ctx, exportertest.NewNopSettings(metadata.Type), expCfg)
			require.NoError(t, err)
			require.NoError(t, exp.Start(ctx, componenttest.NewNopHost()))
			t.Cleanup(func() { require.NoError(t, exp.Shutdown(ctx)) })

			require.NoError(t, exp.ConsumeMetrics(ctx, buildMetricsWithEntities()))

			yamlConfig := `
global:
  scrape_interval: 50ms
scrape_configs:
  - job_name: 'scraper'
    honor_labels: true
    scrape_interval: 50ms
    scrape_timeout: 50ms
    static_configs:
      - targets: ['` + addr + `']
`
			promCfg := new(prometheusreceiver.PromConfig)
			require.NoError(t, yaml.Unmarshal([]byte(yamlConfig), promCfg))

			sink := &consumertest.MetricsSink{}
			rcvCfg := &prometheusreceiver.Config{
				PrometheusConfig: promCfg,
			}
			rcv, err := prometheusreceiver.NewFactory().CreateMetrics(ctx, receivertest.NewNopSettings(metadata.Type), rcvCfg, sink)
			require.NoError(t, err)
			require.NoError(t, rcv.Start(ctx, componenttest.NewNopHost()))
			t.Cleanup(func() { require.NoError(t, rcv.Shutdown(ctx)) })

			var matchedRes pcommon.Resource
			require.EventuallyWithT(t, func(c *assert.CollectT) {
				for _, batch := range sink.AllMetrics() {
					for i := 0; i < batch.ResourceMetrics().Len(); i++ {
						rm := batch.ResourceMetrics().At(i)
						if inst, ok := rm.Resource().Attributes().Get("service.instance.id"); ok && inst.AsString() == "id-1" {
							// Ensure otel_entity_info itself was dropped from metrics.
							for j := 0; j < rm.ScopeMetrics().Len(); j++ {
								ms := rm.ScopeMetrics().At(j).Metrics()
								for k := 0; k < ms.Len(); k++ {
									assert.NotEqual(c, prometheustranslator.EntityInfoMetricName, ms.At(k).Name())
								}
							}
							if entity.ResourceEntityRefs(rm.Resource()).Len() == 2 {
								matchedRes = rm.Resource()
								return
							}
							assert.Failf(c, "resource found without expected entities", "attrs=%v entities=%d", rm.Resource().Attributes().AsRaw(), entity.ResourceEntityRefs(rm.Resource()).Len())
							return
						}
					}
				}
				assert.Failf(c, "waiting for scraped batch with reconstructed entities", "batches=%d", len(sink.AllMetrics()))
			}, 15*time.Second, 50*time.Millisecond)

			refs := entity.ResourceEntityRefs(matchedRes)
			require.Equal(t, 2, refs.Len())
			byType := map[string]entity.EntityRef{}
			for i := 0; i < refs.Len(); i++ {
				er := refs.At(i)
				byType[er.Type()] = er
			}
			require.Contains(t, byType, "service.instance")
			require.Equal(t, "https://opentelemetry.io/schemas/1.31.0", byType["service.instance"].SchemaUrl())
			require.Equal(t, tc.wantSvcIDKeys, byType["service.instance"].IdKeys().AsRaw())
			require.Equal(t, tc.wantSvcDescKeys, byType["service.instance"].DescriptionKeys().AsRaw())

			require.Contains(t, byType, "k8s.pod")
			require.Empty(t, byType["k8s.pod"].SchemaUrl())
			require.Equal(t, tc.wantPodIDKeys, byType["k8s.pod"].IdKeys().AsRaw())
			require.Equal(t, tc.wantPodDescKeys, byType["k8s.pod"].DescriptionKeys().AsRaw())
		})
	}
}
