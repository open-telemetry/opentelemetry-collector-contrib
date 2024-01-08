package testbed

type DataConnector interface {
	// GenConfigYAMLStr generates a config string to place in receiver part of collector config
	// so that it can receive data from this sender.
	GenConfigYAMLStr() string

	// ProtocolName returns exporter name to use in collector config pipeline.
	ProtocolName() string

	// metrics/logs/traces?
	ReceiverPipelineType() string

	ExporterPipelineType() string
}
