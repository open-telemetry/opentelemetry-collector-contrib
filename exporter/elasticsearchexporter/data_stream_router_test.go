// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

type routeTestCase struct {
	name        string
	mode        MappingMode
	scopeName   string
	scopeAttrs  map[string]any
	recordAttrs map[string]any
	want        elasticsearch.Index
}

func createRouteTests(dsType string) []routeTestCase {
	renderWantRoute := func(dsType, dsDataset, dsNamespace string, mode MappingMode) elasticsearch.Index {
		if mode == MappingOTel {
			dsDataset += ".otel"
		}
		return elasticsearch.NewDataStreamIndex(dsType, dsDataset, dsNamespace)
	}

	return []routeTestCase{
		{
			name: "default",
			mode: MappingNone,
			want: renderWantRoute(dsType, defaultDataStreamDataset, defaultDataStreamNamespace, MappingNone),
		},
		{
			name: "otel",
			mode: MappingOTel,
			want: renderWantRoute(dsType, defaultDataStreamDataset, defaultDataStreamNamespace, MappingOTel),
		},
		{
			name:      "otel with elasticsearch.index",
			mode:      MappingOTel,
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/should/be/ignored",
			recordAttrs: map[string]any{
				"elasticsearch.index": "my-index",
			},
			want: elasticsearch.Index{
				Index: "my-index",
			},
		},
		{
			name:      "otel with data_stream attrs",
			mode:      MappingOTel,
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/should/be/ignored",
			recordAttrs: map[string]any{
				"data_stream.dataset":   "foo",
				"data_stream.namespace": "bar",
			},
			want: renderWantRoute(dsType, "foo", "bar", MappingOTel),
		},
		{
			name:      "default with scope-based routing for self-telemetry (sanity)",
			mode:      MappingNone,
			scopeName: "go.opentelemetry.io/collector/receiver/receiverhelper",
			want:      renderWantRoute(dsType, collectorSelfTelemetryDataStreamDataset, defaultDataStreamNamespace, MappingNone),
		},
		{
			name:      "otel with scope-based routing for encoding (sanity)",
			mode:      MappingOTel,
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension",
			scopeAttrs: map[string]any{
				"encoding.format": "aws.cloudtrail",
			},
			want: renderWantRoute(dsType, "aws.cloudtrail", defaultDataStreamNamespace, MappingOTel),
		},
		{
			name:      "otel with scope-based routing for receivers (sanity)",
			mode:      MappingOTel,
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper",
			want:      renderWantRoute(dsType, "hostmetricsreceiver", defaultDataStreamNamespace, MappingOTel),
		},
	}
}

func TestRouteLogRecord(t *testing.T) {
	tests := createRouteTests(defaultDataStreamTypeLogs)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := dynamicDocumentRouter{mode: tc.mode}
			scope := pcommon.NewInstrumentationScope()
			scope.SetName(tc.scopeName)
			fillAttributeMap(scope.Attributes(), tc.scopeAttrs)

			recordAttrMap := pcommon.NewMap()
			fillAttributeMap(recordAttrMap, tc.recordAttrs)

			ds, err := router.routeLogRecord(pcommon.NewResource(), scope, recordAttrMap)
			require.NoError(t, err)
			assert.Equal(t, tc.want, ds)
		})
	}

	t.Run("test data_stream.type for bodymap mode", func(t *testing.T) {
		dsType := "metrics"
		router := dynamicDocumentRouter{mode: MappingBodyMap}
		attrs := pcommon.NewMap()
		attrs.PutStr("data_stream.type", dsType)
		ds, err := router.routeLogRecord(pcommon.NewResource(), pcommon.NewInstrumentationScope(), attrs)
		require.NoError(t, err)
		assert.Equal(t, dsType, ds.Type)
	})
	t.Run("test data_stream.type is not honored for other modes (except bodymap)", func(t *testing.T) {
		dsType := "metrics"
		router := dynamicDocumentRouter{mode: MappingOTel}
		attrs := pcommon.NewMap()
		attrs.PutStr("data_stream.type", dsType)
		ds, err := router.routeLogRecord(pcommon.NewResource(), pcommon.NewInstrumentationScope(), attrs)
		require.NoError(t, err)
		assert.Equal(t, "logs", ds.Type) // should equal to logs
	})

	t.Run("test data_stream.type does not accept values other than logs/metrics", func(t *testing.T) {
		dsType := "random"
		router := dynamicDocumentRouter{mode: MappingBodyMap}
		attrs := pcommon.NewMap()
		attrs.PutStr("data_stream.type", dsType)
		_, err := router.routeLogRecord(pcommon.NewResource(), pcommon.NewInstrumentationScope(), attrs)
		require.Error(t, err, "data_stream.type cannot be other than logs or metrics")
	})
}

func TestRouteDataPoint(t *testing.T) {
	tests := createRouteTests(defaultDataStreamTypeMetrics)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := dynamicDocumentRouter{mode: tc.mode}
			scope := pcommon.NewInstrumentationScope()
			scope.SetName(tc.scopeName)
			fillAttributeMap(scope.Attributes(), tc.scopeAttrs)

			recordAttrMap := pcommon.NewMap()
			fillAttributeMap(recordAttrMap, tc.recordAttrs)

			ds, err := router.routeDataPoint(pcommon.NewResource(), scope, recordAttrMap)
			require.NoError(t, err)
			assert.Equal(t, tc.want, ds)
		})
	}
}

func TestRouteSpan(t *testing.T) {
	tests := createRouteTests(defaultDataStreamTypeTraces)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := dynamicDocumentRouter{mode: tc.mode}
			scope := pcommon.NewInstrumentationScope()
			scope.SetName(tc.scopeName)
			fillAttributeMap(scope.Attributes(), tc.scopeAttrs)

			recordAttrMap := pcommon.NewMap()
			fillAttributeMap(recordAttrMap, tc.recordAttrs)

			ds, err := router.routeSpan(pcommon.NewResource(), scope, recordAttrMap)
			require.NoError(t, err)
			assert.Equal(t, tc.want, ds)
		})
	}
}

func TestApplyRouting(t *testing.T) {
	tests := []struct {
		name        string
		scopeName   string
		scopeAttrs  map[string]any
		wantDataset string
		wantFound   bool
	}{
		{
			name:        "no routing applied for default scope",
			scopeName:   "",
			wantDataset: "",
			wantFound:   false,
		},
		{
			name:        "no routing applied for non-receiver scope name",
			scopeName:   "some_other_scope_name",
			wantDataset: "",
			wantFound:   false,
		},
		{
			name:        "receiver-based routing with hostmetricsreceiver",
			scopeName:   "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper",
			wantDataset: "hostmetricsreceiver",
			wantFound:   true,
		},
		{
			name:        "receiver without a receiver name",
			scopeName:   "some.scope.name/receiver/receiver/should/be/ignored",
			wantDataset: "",
			wantFound:   false,
		},
		{
			name:        "otel collector self-telemetry for receivers",
			scopeName:   "go.opentelemetry.io/collector/receiver/receiverhelper",
			wantDataset: collectorSelfTelemetryDataStreamDataset,
			wantFound:   true,
		},
		{
			name:        "otel collector self-telemetry for scrapers",
			scopeName:   "go.opentelemetry.io/collector/scraper/scraperhelper",
			wantDataset: collectorSelfTelemetryDataStreamDataset,
			wantFound:   true,
		},
		{
			name:        "otel collector self-telemetry for processors",
			scopeName:   "go.opentelemetry.io/collector/processor/processorhelper",
			wantDataset: collectorSelfTelemetryDataStreamDataset,
			wantFound:   true,
		},
		{
			name:        "otel collector self-telemetry for exporters",
			scopeName:   "go.opentelemetry.io/collector/exporter/exporterhelper",
			wantDataset: collectorSelfTelemetryDataStreamDataset,
			wantFound:   true,
		},
		{
			name:        "otel collector self-telemetry for service",
			scopeName:   "go.opentelemetry.io/collector/service",
			wantDataset: collectorSelfTelemetryDataStreamDataset,
			wantFound:   true,
		},
		{
			name:      "encoding-based routing with aws.cloudtrail",
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension",
			scopeAttrs: map[string]any{
				"encoding.format": "aws.cloudtrail",
			},
			wantDataset: "aws.cloudtrail",
			wantFound:   true,
		},
		{
			name:      "encoding-based routing with aws.vpcflow",
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension",
			scopeAttrs: map[string]any{
				"encoding.format": "aws.vpcflow",
			},
			wantDataset: "aws.vpcflow",
			wantFound:   true,
		},
		{
			name:      "encoding-based routing takes precedence over receiver-based routing",
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper",
			scopeAttrs: map[string]any{
				"encoding.format": "aws.vpcflow",
			},
			wantDataset: "aws.vpcflow",
			wantFound:   true,
		},
		{
			name:      "self-telemetry takes precedence over encoding-based routing",
			scopeName: "go.opentelemetry.io/collector/receiver/receiverhelper",
			scopeAttrs: map[string]any{
				"encoding.format": "aws.cloudtrail",
			},
			wantDataset: collectorSelfTelemetryDataStreamDataset,
			wantFound:   true,
		},
		{
			name:      "encoding format that is wrong type is ignored",
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension",
			scopeAttrs: map[string]any{
				"encoding.format": true,
			},
			wantDataset: "",
			wantFound:   false,
		},
		{
			name:      "non-encoding scope attributes are ignored",
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension",
			scopeAttrs: map[string]any{
				"some_other_attr": "should_be_ignored",
			},
			wantDataset: "",
			wantFound:   false,
		},
		{
			name:      "empty encoding.format scope attribute is ignored",
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension",
			scopeAttrs: map[string]any{
				"encoding.format": "",
			},
			wantDataset: "",
			wantFound:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scope := pcommon.NewInstrumentationScope()
			scope.SetName(tc.scopeName)
			fillAttributeMap(scope.Attributes(), tc.scopeAttrs)

			dataset, found := applyScopeRouting(scope)
			assert.Equal(t, tc.wantDataset, dataset)
			assert.Equal(t, tc.wantFound, found)
		})
	}
}
