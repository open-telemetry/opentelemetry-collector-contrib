// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package correctnesstests // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests"

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type ProcessorNameAndConfigBody struct {
	Name string
	Body string
}

// CreateConfigYaml creates a yaml config for an otel collector given a testbed sender, testbed receiver, any
// processors, and a pipeline type. A collector created from the resulting yaml string should be able to talk
// the specified sender and receiver.
func CreateConfigYaml(
	tb testing.TB,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	connector testbed.DataConnector,
	processors []ProcessorNameAndConfigBody,
) string {
	var plannedPorts []int
	if sender != nil {
		if tcpAddr, ok := sender.GetEndpoint().(*net.TCPAddr); ok {
			plannedPorts = append(plannedPorts, tcpAddr.Port)
		}
	}

	// Avoid picking a telemetry port that is already planned for the sender.
	// This prevents port collisions between the collector's Prometheus telemetry
	// and the collector's receivers (sender endpoint), which can happen if CreateConfigYaml
	// is called before the receivers are started (bound).
	telemetryPort := testutil.GetAvailablePort(tb)
	for slices.Contains(plannedPorts, telemetryPort) {
		telemetryPort = testutil.GetAvailablePort(tb)
	}

	// Prepare extra processor config section and comma-separated list of extra processor
	// names to use in corresponding "processors" settings.
	var processorsSectionsBuilder, processorsListBuilder strings.Builder
	if len(processors) > 0 {
		first := true
		for i := range processors {
			processorsSectionsBuilder.WriteString(processors[i].Body + "\n")
			if !first {
				processorsListBuilder.WriteString(",")
			}
			processorsListBuilder.WriteString(processors[i].Name)
			first = false
		}
	}
	processorsSections := processorsSectionsBuilder.String()
	processorsList := processorsListBuilder.String()

	var pipeline1 string
	switch sender.(type) {
	case testbed.TraceDataSender:
		pipeline1 = "traces"
	case testbed.MetricDataSender:
		pipeline1 = "metrics"
	case testbed.LogDataSender:
		pipeline1 = "logs"
	default:
		tb.Error("Invalid DataSender type")
	}

	if connector != nil {
		pipeline2 := connector.GetReceiverType()

		format := `
receivers:%v
exporters:%v
processors:
  %s

extensions:

connectors:%v

service:
  telemetry:
    metrics:
      readers:
        - pull:
            exporter:
              prometheus:
                host: '127.0.0.1'
                port: %d
    logs:
      level: "debug"
  extensions:
  pipelines:
    %s/in:
      receivers: [%v]
      processors: [%s]
      exporters: [%v]
    %s/out:
      receivers: [%v]
      exporters: [%v]
`
		return fmt.Sprintf(
			format,
			sender.GenConfigYAMLStr(),
			receiver.GenConfigYAMLStr(),
			processorsSections,
			connector.GenConfigYAMLStr(),
			telemetryPort,
			pipeline1,
			sender.ProtocolName(),
			processorsList,
			connector.ProtocolName(),
			pipeline2,
			connector.ProtocolName(),
			receiver.ProtocolName(),
		)
	}

	format := `
receivers:%v
exporters:%v
processors:
  %s

extensions:

service:
  telemetry:
    metrics:
      readers:
        - pull:
            exporter:
              prometheus:
                host: '127.0.0.1'
                port: %d
    logs:
      level: "debug"
  extensions:
  pipelines:
    %s:
      receivers: [%v]
      processors: [%s]
      exporters: [%v]
`

	return fmt.Sprintf(
		format,
		sender.GenConfigYAMLStr(),
		receiver.GenConfigYAMLStr(),
		processorsSections,
		telemetryPort,
		pipeline1,
		sender.ProtocolName(),
		processorsList,
		receiver.ProtocolName(),
	)
}

// PipelineDef holds the information necessary to run a single testbed configuration.
type PipelineDef struct {
	Receiver      string
	Exporter      string
	Connector     string
	TestName      string
	DataSender    testbed.DataSender
	DataReceiver  testbed.DataReceiver
	DataConnector testbed.DataConnector
	ResourceSpec  testbed.ResourceSpec
}

// LoadPictOutputPipelineDefs generates a slice of PipelineDefs from the passed-in generated PICT file. The
// result should be a set of PipelineDefs that covers all possible pipeline configurations.
func LoadPictOutputPipelineDefs(fileName string) ([]PipelineDef, error) {
	file, err := os.Open(filepath.Clean(fileName))
	if err != nil {
		return nil, err
	}
	defer func() {
		cerr := file.Close()
		if err == nil {
			err = cerr
		}
	}()

	var defs []PipelineDef
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		s := strings.Split(scanner.Text(), "\t")
		if s[0] == "Receiver" {
			continue
		}

		defs = append(defs, PipelineDef{Receiver: s[0], Exporter: s[1]})
	}

	return defs, err
}

