package prometheusremotewrite

import (
	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"testing"
)

func TestPrometheusConverter_getOrCreateTimeSeriesV2(t *testing.T) {
	converter := newPrometheusConverterV2()
	lbls := labels.Labels{
		labels.Label{
			Name:  "key1",
			Value: "value1",
		},
		labels.Label{
			Name:  "key2",
			Value: "value2",
		},
	}
	ts, created := converter.getOrCreateTimeSeries(lbls)
	require.NotNil(t, ts)
	require.True(t, created)

	var b labels.ScratchBuilder
	createdLabels := ts.ToLabels(&b, converter.symbolTable.Symbols())

	// Now, get (not create) the unique time series
	gotTS, created := converter.getOrCreateTimeSeries(createdLabels)
	require.Same(t, ts, gotTS)
	require.False(t, created)

	var keys []uint64
	for k := range converter.unique {
		keys = append(keys, k)
	}
	require.Len(t, keys, 1)
	h := keys[0]

	// Make sure that state is correctly set
	require.Equal(t, map[uint64]*writev2.TimeSeries{
		h: ts,
	}, converter.unique)
	require.Empty(t, converter.conflicts)

	// Fake a hash collision, by making this not equal to the next series with the same hash
	createdLabels = append(createdLabels, labels.Label{Name: "key3", Value: "value3"})
	ts.LabelsRefs = converter.symbolTable.SymbolizeLabels(createdLabels, ts.LabelsRefs)

	// Make the first hash collision
	cTS1, created := converter.getOrCreateTimeSeries(lbls)
	require.NotNil(t, cTS1)
	require.True(t, created)
	require.Equal(t, map[uint64][]*writev2.TimeSeries{
		h: {cTS1},
	}, converter.conflicts)

	// Fake a hash collision, by making this not equal to the next series with the same hash
	createdLabels1 := cTS1.ToLabels(&b, converter.symbolTable.Symbols())
	createdLabels1 = append(createdLabels1, labels.Label{Name: "key3", Value: "value3"})
	cTS1.LabelsRefs = converter.symbolTable.SymbolizeLabels(createdLabels1, ts.LabelsRefs)

	// Make the second hash collision
	cTS2, created := converter.getOrCreateTimeSeries(lbls)
	require.NotNil(t, cTS2)
	require.True(t, created)
	require.Equal(t, map[uint64][]*writev2.TimeSeries{
		h: {cTS1, cTS2},
	}, converter.conflicts)

	// Now, get (not create) the second colliding time series
	gotCTS2, created := converter.getOrCreateTimeSeries(lbls)
	require.Same(t, cTS2, gotCTS2)
	require.False(t, created)
	require.Equal(t, map[uint64][]*writev2.TimeSeries{
		h: {cTS1, cTS2},
	}, converter.conflicts)

	require.Equal(t, map[uint64]*writev2.TimeSeries{
		h: ts,
	}, converter.unique)
}

// Test_createLabelSet checks resultant label names are sanitized and label in extra overrides label in labels if
// collision happens. It does not check whether labels are not sorted
func Test_createLabelSetV2(t *testing.T) {
	tests := []struct {
		name           string
		resource       pcommon.Resource
		orig           pcommon.Map
		externalLabels map[string]string
		extras         []string
		want           labels.Labels
	}{
		{
			"labels_clean",
			pcommon.NewResource(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label11, value11, label12, value12, label31, value31, label32, value32),
		},
		{
			"labels_with_resource",
			func() pcommon.Resource {
				res := pcommon.NewResource()
				res.Attributes().PutStr("service.name", "prometheus")
				res.Attributes().PutStr("service.instance.id", "127.0.0.1:8080")
				return res
			}(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label11, value11, label12, value12, label31, value31, label32, value32, "job", "prometheus", "instance", "127.0.0.1:8080"),
		},
		{
			"labels_with_nonstring_resource",
			func() pcommon.Resource {
				res := pcommon.NewResource()
				res.Attributes().PutInt("service.name", 12345)
				res.Attributes().PutBool("service.instance.id", true)
				return res
			}(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label11, value11, label12, value12, label31, value31, label32, value32, "job", "12345", "instance", "true"),
		},
		{
			"labels_duplicate_in_extras",
			pcommon.NewResource(),
			lbs1,
			map[string]string{},
			[]string{label11, value31},
			getPromLabelsV2(label11, value31, label12, value12),
		},
		{
			"labels_dirty",
			pcommon.NewResource(),
			lbs1Dirty,
			map[string]string{},
			[]string{label31 + dirty1, value31, label32, value32},
			getPromLabelsV2(label11+"_", value11, "key_"+label12, value12, label31+"_", value31, label32, value32),
		},
		{
			"no_original_case",
			pcommon.NewResource(),
			pcommon.NewMap(),
			nil,
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label31, value31, label32, value32),
		},
		{
			"empty_extra_case",
			pcommon.NewResource(),
			lbs1,
			map[string]string{},
			[]string{"", ""},
			getPromLabelsV2(label11, value11, label12, value12, "", ""),
		},
		{
			"single_left_over_case",
			pcommon.NewResource(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32},
			getPromLabelsV2(label11, value11, label12, value12, label31, value31),
		},
		{
			"valid_external_labels",
			pcommon.NewResource(),
			lbs1,
			exlbs1,
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label11, value11, label12, value12, label41, value41, label31, value31, label32, value32),
		},
		{
			"overwritten_external_labels",
			pcommon.NewResource(),
			lbs1,
			exlbs2,
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label11, value11, label12, value12, label31, value31, label32, value32),
		},
		{
			"colliding attributes",
			pcommon.NewResource(),
			lbsColliding,
			nil,
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(collidingSanitized, value11+";"+value12, label31, value31, label32, value32),
		},
		{
			"sanitize_labels_starts_with_underscore",
			pcommon.NewResource(),
			lbs3,
			exlbs1,
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label11, value11, label12, value12, "key"+label51, value51, label41, value41, label31, value31, label32, value32),
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := createAttributesV2(tt.resource, tt.orig, tt.externalLabels, nil, true, tt.extras...)
			assert.ElementsMatch(t, tt.want, res)
		})
	}
}
