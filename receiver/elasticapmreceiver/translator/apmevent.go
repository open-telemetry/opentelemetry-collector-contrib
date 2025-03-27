package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func parseBaseEvent(event *modelpb.APMEvent, attrs pcommon.Map) {
	if event == nil {
		return
	}

	PutNumericLabelValue(attrs, event.NumericLabels)
	PutLabelValue(attrs, event.Labels)
	parseCloud(event.Cloud, attrs)
	parseService(event.Service, attrs)
	parseFaas(event.Faas, attrs)
	parseNetwork(event.Network, attrs)
	parseContainer(event.Container, attrs)
	parseUser(event.User, attrs)
	parseDevice(event.Device, attrs)
	parseKubernetes(event.Kubernetes, attrs)
	parseObserver(event.Observer, attrs)
	parseDataStream(event.DataStream, attrs)
	parseAgent(event.Agent, attrs)
	parseHTTP(event.Http, attrs)
	parseUserAgent(event.UserAgent, attrs)

	parseHost(event.Host, attrs)
	parseURL(event.Url, attrs)
	parseLog(event.Log, attrs)
	parseSource(event.Source, attrs)
	parseClient(event.Client, attrs)

	parseDestination(event.Destination, attrs)
	parseSession(event.Session, attrs)
	parseProcess(event.Process, attrs)
	parseEvent(event.Event, attrs)
}

func ConvertMetadata(event *modelpb.APMEvent, resource pcommon.Resource) {
	if event == nil {
		return
	}

	attrs := resource.Attributes()
	parseBaseEvent(event, attrs)
}
