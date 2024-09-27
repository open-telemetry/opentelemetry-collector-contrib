package elasticsearchexporter

import "go.opentelemetry.io/collector/pdata/pcommon"

const (
	mappingHintsAttrKey = "elasticsearch.mapping.hints"
)

type mappingHint string

const (
	hintAggregateMetricDouble mappingHint = "aggregate_metric_double"
	hintDocCount              mappingHint = "_doc_count"
)

func hasHint(attrs pcommon.Map, hint mappingHint) (found bool) {
	attrs.Range(func(k string, v pcommon.Value) bool {
		if k != mappingHintsAttrKey || v.Type() != pcommon.ValueTypeSlice {
			return true
		}
		slice := v.Slice()
		for i := 0; i < slice.Len(); i++ {
			if slice.At(i).Str() == string(hint) {
				found = true
				return false
			}
		}
		return true
	})
	return
}
