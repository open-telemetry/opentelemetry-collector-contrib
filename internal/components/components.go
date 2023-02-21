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

package components // import "github.com/asserts/opentelemetry-collector-contrib/internal/components"

import (
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/loggingexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/ballastextension"
	"go.opentelemetry.io/collector/extension/zpagesextension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/asserts/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/awskinesisexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/awsxrayexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/azuremonitorexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/carbonexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/clickhouseexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/coralogixexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/datadogexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/dynatraceexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/f5cloudexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/fileexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/googlecloudexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/influxdbexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/instanaexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/jaegerexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/loadbalancingexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/logicmonitorexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/logzioexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/lokiexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/mezmoexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/opencensusexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/parquetexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/pulsarexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/sapmexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/sentryexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/signalfxexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/skywalkingexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/splunkhecexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/sumologicexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter"
	"github.com/asserts/opentelemetry-collector-contrib/exporter/zipkinexporter"
	"github.com/asserts/opentelemetry-collector-contrib/extension/asapauthextension"
	"github.com/asserts/opentelemetry-collector-contrib/extension/awsproxy"
	"github.com/asserts/opentelemetry-collector-contrib/extension/basicauthextension"
	"github.com/asserts/opentelemetry-collector-contrib/extension/bearertokenauthextension"
	"github.com/asserts/opentelemetry-collector-contrib/extension/headerssetterextension"
	"github.com/asserts/opentelemetry-collector-contrib/extension/healthcheckextension"
	"github.com/asserts/opentelemetry-collector-contrib/extension/httpforwarder"
	"github.com/asserts/opentelemetry-collector-contrib/extension/oauth2clientauthextension"
	"github.com/asserts/opentelemetry-collector-contrib/extension/observer/ecstaskobserver"
	"github.com/asserts/opentelemetry-collector-contrib/extension/observer/hostobserver"
	"github.com/asserts/opentelemetry-collector-contrib/extension/observer/k8sobserver"
	"github.com/asserts/opentelemetry-collector-contrib/extension/oidcauthextension"
	"github.com/asserts/opentelemetry-collector-contrib/extension/pprofextension"
	"github.com/asserts/opentelemetry-collector-contrib/extension/sigv4authextension"
	"github.com/asserts/opentelemetry-collector-contrib/extension/storage/dbstorage"
	"github.com/asserts/opentelemetry-collector-contrib/extension/storage/filestorage"
	"github.com/asserts/opentelemetry-collector-contrib/processor/attributesprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/datadogprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/deltatorateprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/groupbytraceprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/metricsgenerationprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/metricstransformprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/routingprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/servicegraphprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/spanmetricsprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/spanprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/aerospikereceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/apachereceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/awsxrayreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/azureblobreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/bigipreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/carbonreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/chronyreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/collectdreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/couchdbreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/datadogreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/elasticsearchreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/expvarreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/filelogreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/haproxyreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/httpcheckreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/iisreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/influxdbreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/jaegerreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/jmxreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/k8sclusterreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/k8seventsreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/kafkareceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/memcachedreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/mongodbreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/mysqlreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/nginxreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/nsxtreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/opencensusreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/oracledbreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/podmanreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/prometheusexecreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/pulsarreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/purefareceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/purefbreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/receivercreator"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/redisreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/riakreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/saphanareceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/sapmreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/signalfxreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/skywalkingreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/snmpreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/solacereceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/splunkhecreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/sqlserverreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/sshcheckreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/statsdreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/syslogreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/tcplogreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/udplogreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/vcenterreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/wavefrontreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/windowseventlogreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/zipkinreceiver"
	"github.com/asserts/opentelemetry-collector-contrib/receiver/zookeeperreceiver"
)

func Components() (otelcol.Factories, error) {
	var err error
	factories := otelcol.Factories{}
	extensions := []extension.Factory{
		asapauthextension.NewFactory(),
		awsproxy.NewFactory(),
		ballastextension.NewFactory(),
		basicauthextension.NewFactory(),
		bearertokenauthextension.NewFactory(),
		dbstorage.NewFactory(),
		ecstaskobserver.NewFactory(),
		filestorage.NewFactory(),
		headerssetterextension.NewFactory(),
		healthcheckextension.NewFactory(),
		hostobserver.NewFactory(),
		httpforwarder.NewFactory(),
		k8sobserver.NewFactory(),
		pprofextension.NewFactory(),
		oauth2clientauthextension.NewFactory(),
		oidcauthextension.NewFactory(),
		sigv4authextension.NewFactory(),
		zpagesextension.NewFactory(),
	}
	factories.Extensions, err = extension.MakeFactoryMap(extensions...)
	if err != nil {
		return otelcol.Factories{}, err
	}

	receivers := []receiver.Factory{
		activedirectorydsreceiver.NewFactory(),
		aerospikereceiver.NewFactory(),
		apachereceiver.NewFactory(),
		awscontainerinsightreceiver.NewFactory(),
		awsecscontainermetricsreceiver.NewFactory(),
		awsfirehosereceiver.NewFactory(),
		awscloudwatchreceiver.NewFactory(),
		awsxrayreceiver.NewFactory(),
		azureblobreceiver.NewFactory(),
		azureeventhubreceiver.NewFactory(),
		bigipreceiver.NewFactory(),
		carbonreceiver.NewFactory(),
		chronyreceiver.NewFactory(),
		cloudfoundryreceiver.NewFactory(),
		collectdreceiver.NewFactory(),
		couchdbreceiver.NewFactory(),
		datadogreceiver.NewFactory(),
		dockerstatsreceiver.NewFactory(),
		dotnetdiagnosticsreceiver.NewFactory(),
		elasticsearchreceiver.NewFactory(),
		expvarreceiver.NewFactory(),
		filelogreceiver.NewFactory(),
		flinkmetricsreceiver.NewFactory(),
		fluentforwardreceiver.NewFactory(),
		googlecloudspannerreceiver.NewFactory(),
		googlecloudpubsubreceiver.NewFactory(),
		haproxyreceiver.NewFactory(),
		hostmetricsreceiver.NewFactory(),
		httpcheckreceiver.NewFactory(),
		influxdbreceiver.NewFactory(),
		iisreceiver.NewFactory(),
		jaegerreceiver.NewFactory(),
		jmxreceiver.NewFactory(),
		journaldreceiver.NewFactory(),
		kafkareceiver.NewFactory(),
		kafkametricsreceiver.NewFactory(),
		k8sclusterreceiver.NewFactory(),
		k8seventsreceiver.NewFactory(),
		k8sobjectsreceiver.NewFactory(),
		kubeletstatsreceiver.NewFactory(),
		memcachedreceiver.NewFactory(),
		mongodbatlasreceiver.NewFactory(),
		mongodbreceiver.NewFactory(),
		mysqlreceiver.NewFactory(),
		nsxtreceiver.NewFactory(),
		nginxreceiver.NewFactory(),
		opencensusreceiver.NewFactory(),
		oracledbreceiver.NewFactory(),
		otlpjsonfilereceiver.NewFactory(),
		otlpreceiver.NewFactory(),
		podmanreceiver.NewFactory(),
		postgresqlreceiver.NewFactory(),
		prometheusexecreceiver.NewFactory(),
		prometheusreceiver.NewFactory(),
		pulsarreceiver.NewFactory(),
		purefareceiver.NewFactory(),
		purefbreceiver.NewFactory(),
		rabbitmqreceiver.NewFactory(),
		receivercreator.NewFactory(),
		redisreceiver.NewFactory(),
		riakreceiver.NewFactory(),
		saphanareceiver.NewFactory(),
		sapmreceiver.NewFactory(),
		signalfxreceiver.NewFactory(),
		simpleprometheusreceiver.NewFactory(),
		skywalkingreceiver.NewFactory(),
		snmpreceiver.NewFactory(),
		solacereceiver.NewFactory(),
		splunkhecreceiver.NewFactory(),
		sqlqueryreceiver.NewFactory(),
		sqlserverreceiver.NewFactory(),
		sshcheckreceiver.NewFactory(),
		statsdreceiver.NewFactory(),
		wavefrontreceiver.NewFactory(),
		windowseventlogreceiver.NewFactory(),
		windowsperfcountersreceiver.NewFactory(),
		zookeeperreceiver.NewFactory(),
		syslogreceiver.NewFactory(),
		tcplogreceiver.NewFactory(),
		udplogreceiver.NewFactory(),
		vcenterreceiver.NewFactory(),
		zipkinreceiver.NewFactory(),
	}
	factories.Receivers, err = receiver.MakeFactoryMap(receivers...)
	if err != nil {
		return otelcol.Factories{}, err
	}

	exporters := []exporter.Factory{
		alibabacloudlogserviceexporter.NewFactory(),
		awscloudwatchlogsexporter.NewFactory(),
		awsemfexporter.NewFactory(),
		awskinesisexporter.NewFactory(),
		awsxrayexporter.NewFactory(),
		azuredataexplorerexporter.NewFactory(),
		azuremonitorexporter.NewFactory(),
		carbonexporter.NewFactory(),
		clickhouseexporter.NewFactory(),
		coralogixexporter.NewFactory(),
		datadogexporter.NewFactory(),
		dynatraceexporter.NewFactory(),
		elasticsearchexporter.NewFactory(),
		f5cloudexporter.NewFactory(),
		fileexporter.NewFactory(),
		googlecloudexporter.NewFactory(),
		googlemanagedprometheusexporter.NewFactory(),
		googlecloudpubsubexporter.NewFactory(),
		influxdbexporter.NewFactory(),
		instanaexporter.NewFactory(),
		jaegerexporter.NewFactory(),
		jaegerthrifthttpexporter.NewFactory(),
		kafkaexporter.NewFactory(),
		loadbalancingexporter.NewFactory(),
		loggingexporter.NewFactory(),
		logicmonitorexporter.NewFactory(),
		logzioexporter.NewFactory(),
		lokiexporter.NewFactory(),
		mezmoexporter.NewFactory(),
		opencensusexporter.NewFactory(),
		otlpexporter.NewFactory(),
		otlphttpexporter.NewFactory(),
		parquetexporter.NewFactory(),
		prometheusexporter.NewFactory(),
		prometheusremotewriteexporter.NewFactory(),
		pulsarexporter.NewFactory(),
		sapmexporter.NewFactory(),
		sentryexporter.NewFactory(),
		signalfxexporter.NewFactory(),
		skywalkingexporter.NewFactory(),
		splunkhecexporter.NewFactory(),
		sumologicexporter.NewFactory(),
		tanzuobservabilityexporter.NewFactory(),
		tencentcloudlogserviceexporter.NewFactory(),
		zipkinexporter.NewFactory(),
	}
	factories.Exporters, err = exporter.MakeFactoryMap(exporters...)
	if err != nil {
		return otelcol.Factories{}, err
	}

	processors := []processor.Factory{
		attributesprocessor.NewFactory(),
		batchprocessor.NewFactory(),
		filterprocessor.NewFactory(),
		groupbyattrsprocessor.NewFactory(),
		groupbytraceprocessor.NewFactory(),
		k8sattributesprocessor.NewFactory(),
		memorylimiterprocessor.NewFactory(),
		metricstransformprocessor.NewFactory(),
		metricsgenerationprocessor.NewFactory(),
		probabilisticsamplerprocessor.NewFactory(),
		resourcedetectionprocessor.NewFactory(),
		resourceprocessor.NewFactory(),
		routingprocessor.NewFactory(),
		tailsamplingprocessor.NewFactory(),
		servicegraphprocessor.NewFactory(),
		spanmetricsprocessor.NewFactory(),
		spanprocessor.NewFactory(),
		cumulativetodeltaprocessor.NewFactory(),
		datadogprocessor.NewFactory(),
		deltatorateprocessor.NewFactory(),
		transformprocessor.NewFactory(),
	}
	factories.Processors, err = processor.MakeFactoryMap(processors...)
	if err != nil {
		return otelcol.Factories{}, err
	}

	return factories, nil
}
