// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

type DataConnector interface {
	// GenConfigYAMLStr generates a config string to place in receiver part of collector config
	// so that it can receive data from this sender.
	GenConfigYAMLStr() string

	// ProtocolName returns exporter name to use in collector config pipeline.
	ProtocolName() string

	// GetReceiverType returns the data type for the DataReceiver in the second pipeline when using connectors
	GetReceiverType() string
}

// DataReceiverBase implement basic functions needed by all receivers.
type DataConnectorBase struct {
	// The data type of the receiver in second pipeline.
	ReceiverDataType string
}
