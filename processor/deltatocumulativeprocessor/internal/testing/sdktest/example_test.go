// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdktest

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdk "go.opentelemetry.io/otel/sdk/metric"
)

// The output of [Test] and [Compare] is similar to the following:
//
//	  []metricdata.Metrics{
//	- 	{
//	- 		Name: "not.exist",
//	- 		Data: metricdata.Sum[float64]{
//	- 			DataPoints:  []metricdata.DataPoint[float64]{{...}},
//	- 			Temporality: s"CumulativeTemporality",
//	- 		},
//	- 	},
//	  	{
//	  		Name:        "requests.total",
//	  		Description: "I will be inherited",
//	  		Unit:        "",
//	  		Data: metricdata.Sum[int64]{
//	  			DataPoints: []metricdata.DataPoint[int64](Inverse(sdktest.Transform.int64, []sdktest.DataPoint[int64]{
//	  				{DataPoint: {StartTime: s"2024-10-11 11:23:37.966150738 +0200 CEST m=+0.001489569", Time: s"2024-10-11 11:23:37.966174238 +0200 CEST m=+0.001513070", Value: 20, ...}, Attributes: {}},
//	  				{
//	  					DataPoint: metricdata.DataPoint[int64]{
//	  						... // 1 ignored field
//	  						StartTime: s"2024-10-11 11:23:37.966150738 +0200 CEST m=+0.001489569",
//	  						Time:      s"2024-10-11 11:23:37.966174238 +0200 CEST m=+0.001513070",
//	- 						Value:     4,
//	+ 						Value:     3,
//	  						Exemplars: nil,
//	  					},
//	  					Attributes: {"error": string("limit")},
//	  				},
//	  			})),
//	  			Temporality: s"CumulativeTemporality",
//	  			IsMonotonic: true,
//	  		},
//	  	},
//	  }
//
// Which is used as follows:
func Example() {
	var spec Spec
	_ = Unmarshal([]byte(`
gauge streams.tracked:
  - int: 40

counter requests.total:
  - int: 20
  - int: 4
    attr: {error: "limit"}

updown not.exist:
  - float: 33.3
`), &spec)

	mr := sdk.NewManualReader()
	meter := sdk.NewMeterProvider(sdk.WithReader(mr)).Meter("test")

	ctx := context.TODO()

	gauge, _ := meter.Int64Gauge("streams.tracked")
	gauge.Record(ctx, 40)

	count, _ := meter.Int64Counter("requests.total", metric.WithDescription("I will be inherited"))
	count.Add(ctx, 20)
	count.Add(ctx, 3, metric.WithAttributes(attribute.String("error", "limit")))

	err := Test(spec, mr)
	fmt.Println(err)
}
