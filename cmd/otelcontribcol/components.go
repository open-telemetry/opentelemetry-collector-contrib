// Copyright 2019 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/service/defaultcomponents"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kinesisexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/newrelicexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver"
)

func components() (component.Factories, error) {
	var errs []error
	factories, err := defaultcomponents.Components()
	if err != nil {
		return component.Factories{}, err
	}

	extensions := []component.ExtensionFactory{
		hostobserver.NewFactory(),
		httpforwarder.NewFactory(),
		k8sobserver.NewFactory(),
	}

	for _, ext := range factories.Extensions {
		extensions = append(extensions, ext)
	}

	factories.Extensions, err = component.MakeExtensionFactoryMap(extensions...)
	if err != nil {
		errs = append(errs, err)
	}

	receivers := []component.ReceiverFactory{
		awsecscontainermetricsreceiver.NewFactory(),
		awsxrayreceiver.NewFactory(),
		carbonreceiver.NewFactory(),
		collectdreceiver.NewFactory(),
		dockerstatsreceiver.NewFactory(),
		k8sclusterreceiver.NewFactory(),
		kubeletstatsreceiver.NewFactory(),
		prometheusexecreceiver.NewFactory(),
		receivercreator.NewFactory(),
		redisreceiver.NewFactory(),
		sapmreceiver.NewFactory(),
		signalfxreceiver.NewFactory(),
		simpleprometheusreceiver.NewFactory(),
		splunkhecreceiver.NewFactory(),
		statsdreceiver.NewFactory(),
		wavefrontreceiver.NewFactory(),
	}
	for _, rcv := range factories.Receivers {
		receivers = append(receivers, rcv)
	}
	factories.Receivers, err = component.MakeReceiverFactoryMap(receivers...)
	if err != nil {
		errs = append(errs, err)
	}

	exporters := []component.ExporterFactory{
		alibabacloudlogserviceexporter.NewFactory(),
		awsemfexporter.NewFactory(),
		awsxrayexporter.NewFactory(),
		azuremonitorexporter.NewFactory(),
		carbonexporter.NewFactory(),
		elasticexporter.NewFactory(),
		honeycombexporter.NewFactory(),
		jaegerthrifthttpexporter.NewFactory(),
		kinesisexporter.NewFactory(),
		newrelicexporter.NewFactory(),
		sapmexporter.NewFactory(),
		sentryexporter.NewFactory(),
		signalfxexporter.NewFactory(),
		splunkhecexporter.NewFactory(),
		stackdriverexporter.NewFactory(),
	}
	for _, exp := range factories.Exporters {
		exporters = append(exporters, exp)
	}
	factories.Exporters, err = component.MakeExporterFactoryMap(exporters...)
	if err != nil {
		errs = append(errs, err)
	}

	processors := []component.ProcessorFactory{
		groupbytraceprocessor.NewFactory(),
		k8sprocessor.NewFactory(),
		metricstransformprocessor.NewFactory(),
		resourcedetectionprocessor.NewFactory(),
		routingprocessor.NewFactory(),
	}
	for _, pr := range factories.Processors {
		processors = append(processors, pr)
	}
	factories.Processors, err = component.MakeProcessorFactoryMap(processors...)
	if err != nil {
		errs = append(errs, err)
	}

	return factories, componenterror.CombineErrors(errs)
}
