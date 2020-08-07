// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build integration

package main

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/contrib/sdk/dynamicconfig/metric/controller/push"

	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/kv"
	"go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Initializes an OTLP exporter and metric provider
func initProvider() (*otlp.Exporter, *push.Controller) {

	exp, err := otlp.NewExporter(
		otlp.WithInsecure(),
		otlp.WithAddress("localhost:55680"),
	)
	handleErr(err, "failed to create exporter")

	resource := resource.New(kv.String("R", "V"))

	pusher := push.New(
		simple.NewWithExactDistribution(),
		exp,
		"localhost:55700",
		push.WithResource(resource),
	)
	global.SetMeterProvider(pusher.Provider())
	pusher.Start()

	return exp, pusher
}

func main() {
	exp, pusher := initProvider()
	defer func() { handleErr(exp.Stop(), "Failed to stop exporter") }()
	defer pusher.Stop() // pushes any last exports to the receiver

	meter := pusher.Provider().Meter("test-meter")
	labels := []kv.KeyValue{kv.Bool("test", true)}

	valuerecorder := metric.Must(meter).
		NewFloat64Counter(
			"an_important_metric",
			metric.WithDescription("Measures the cumulative epicness of the app"),
		).Bind(labels...)
	defer valuerecorder.Unbind()

	for {
		valuerecorder.Add(context.Background(), 1.0)
		time.Sleep(time.Millisecond * 500)
	}

}

func handleErr(err error, message string) {
	if err != nil {
		log.Fatalf("%s: %v", message, err)
	}
}
