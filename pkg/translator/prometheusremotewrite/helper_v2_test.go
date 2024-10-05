package prometheusremotewrite

import (
	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/require"
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
