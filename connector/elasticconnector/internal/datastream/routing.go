package datastream

import "go.opentelemetry.io/collector/pdata/pcommon"

type DataStreamType string

const (
	Metrics DataStreamType = "metrics"
	Logs    DataStreamType = "logs"
	APM     DataStreamType = "apm"
)

var scopeNameToDataset = map[string]string{
	"otelcol/hostmetricsreceiver/cpu":        "system.cpu",
	"otelcol/hostmetricsreceiver/disk":       "system.diskio",
	"otelcol/hostmetricsreceiver/load":       "system.load",
	"otelcol/hostmetricsreceiver/filesystem": "filesystem",
	"otelcol/hostmetricsreceiver/memory":     "system.memory",
	"otelcol/hostmetricsreceiver/network":    "system.network",
	"otelcol/hostmetricsreceiver/paging":     "system.memory",
	"otelcol/hostmetricsreceiver/processes":  "system.process",
	"otelcol/hostmetricsreceiver/process":    "system.process",
}

func AddDataStreamFields(dataStreamType DataStreamType, scope pcommon.InstrumentationScope) error {
	if dataset, ok := scopeNameToDataset[scope.Name()]; ok {
		scopeAttrs := scope.Attributes()
		scopeAttrs.PutStr("data_stream.type", string(dataStreamType))
		scopeAttrs.PutStr("data_stream.dataset", dataset)
	}

	return nil
}
