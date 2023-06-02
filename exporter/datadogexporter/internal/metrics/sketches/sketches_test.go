// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Parts of this package are based on the code from the datadog-agent,
// https://github.com/DataDog/datadog-agent/blob/main/pkg/metrics/sketch_series.go

package sketches // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics/sketches"

import (
	"fmt"
	"testing"

	"github.com/DataDog/agent-payload/v5/gogen"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/quantile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makesketch(n int) *quantile.Sketch {
	s, c := &quantile.Sketch{}, quantile.Default()
	for i := 0; i < n; i++ {
		s.Insert(c, float64(i))
	}
	return s
}

// Makeseries creates a SketchSeries with i+5 Sketch Points
func Makeseries(i int) SketchSeries {
	// Makeseries is deterministic so that we can test for mutation.
	ss := SketchSeries{
		Name: fmt.Sprintf("name.%d", i),
		Tags: []string{
			fmt.Sprintf("a:%d", i),
			fmt.Sprintf("b:%d", i),
		},
		Host:     fmt.Sprintf("host.%d", i),
		Interval: int64(i),
	}

	// We create i+5 Sketch Points to ensure all hosts have at least 5 Sketch Points for tests
	for j := 0; j < i+5; j++ {
		ss.Points = append(ss.Points, SketchPoint{
			Ts:     10 * int64(j),
			Sketch: makesketch(j),
		})
	}

	return ss
}

func check(t *testing.T, in SketchPoint, pb gogen.SketchPayload_Sketch_Dogsketch) {
	t.Helper()
	s, b := in.Sketch, in.Sketch.Basic
	require.Equal(t, in.Ts, pb.Ts)

	// sketch
	k, n := s.Cols()
	require.Equal(t, k, pb.K)
	require.Equal(t, n, pb.N)

	// summary
	require.Equal(t, b.Cnt, pb.Cnt)
	require.Equal(t, b.Min, pb.Min)
	require.Equal(t, b.Max, pb.Max)
	require.Equal(t, b.Avg, pb.Avg)
	require.Equal(t, b.Sum, pb.Sum)
}

func TestSketchSeriesListMarshal(t *testing.T) {
	sl := make(SketchSeriesList, 2)

	for i := range sl {
		sl[i] = Makeseries(i)
	}

	b, err := sl.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pl := new(gogen.SketchPayload)
	if err := pl.Unmarshal(b); err != nil {
		t.Fatal(err)
	}

	require.Len(t, pl.Sketches, len(sl))

	for i, pb := range pl.Sketches {
		in := sl[i]
		require.Equal(t, Makeseries(i), in, "make sure we don't modify input")

		assert.Equal(t, in.Host, pb.Host)
		assert.Equal(t, in.Name, pb.Metric)
		assert.Equal(t, in.Tags, pb.Tags)
		assert.Len(t, pb.Distributions, 0)

		require.Len(t, pb.Dogsketches, len(in.Points))
		for j, pointPb := range pb.Dogsketches {

			check(t, in.Points[j], pointPb)
		}
	}
}
