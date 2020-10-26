// Copyright 2020, OpenTelemetry Authors
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

package jmxreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

const (
	typeStr            = "jmx"
	otlpExporter       = "otlp"
	otlpEndpoint       = "localhost:55680"
	prometheusExporter = "prometheus"
	prometheusEndpoint = "localhost"
	prometheusPort     = 9090
)

func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createReceiver))
}

func createDefaultConfig() configmodels.Receiver {
	return &config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		JARPath:        "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
		Interval:       10 * time.Second,
		Exporter:       otlpExporter,
		OTLPEndpoint:   otlpEndpoint,
		OTLPTimeout:    5 * time.Second,
		PrometheusHost: prometheusEndpoint,
		PrometheusPort: prometheusPort,
	}
}

func createReceiver(
	_ context.Context,
	params component.ReceiverCreateParams,
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumer,
) (component.MetricsReceiver, error) {
	jmxConfig := cfg.(*config)
	if err := jmxConfig.validate(); err != nil {
		return nil, err
	}
	return newJMXMetricReceiver(params.Logger, jmxConfig, consumer), nil
}
