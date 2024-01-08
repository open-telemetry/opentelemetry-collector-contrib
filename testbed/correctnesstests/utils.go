// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package correctnesstests // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests"

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// CreateConfigYaml creates a yaml config for an otel collector given a testbed sender, testbed receiver, any
// processors, and a pipeline type. A collector created from the resulting yaml string should be able to talk
// the specified sender and receiver.
func CreateConfigYaml(
	t testing.TB,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	processors map[string]string,
	pipelineType string,
) string {

	// Prepare extra processor config section and comma-separated list of extra processor
	// names to use in corresponding "processors" settings.
	processorsSections := ""
	processorsList := ""
	if len(processors) > 0 {
		first := true
		for name, cfg := range processors {
			processorsSections += cfg + "\n"
			if !first {
				processorsList += ","
			}
			processorsList += name
			first = false
		}
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
      address: 127.0.0.1:%d
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
		testbed.GetAvailablePort(t),
		pipelineType,
		sender.ProtocolName(),
		processorsList,
		receiver.ProtocolName(),
	)
}

// PipelineDef holds the information necessary to run a single testbed configuration.
type PipelineDef struct {
	Receiver     string
	Exporter     string
	TestName     string
	DataSender   testbed.DataSender
	DataReceiver testbed.DataReceiver
	ResourceSpec testbed.ResourceSpec
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

// ConstructTraceSender creates a testbed trace sender from the passed-in trace sender identifier.
func ConstructTraceSender(t *testing.T, receiver string) testbed.DataSender {
	var sender testbed.DataSender
	switch receiver {
	case "otlp":
		sender = testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t))
	case "opencensus":
		sender = datasenders.NewOCTraceDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t))
	case "jaeger":
		sender = datasenders.NewJaegerGRPCDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t))
	case "zipkin":
		sender = datasenders.NewZipkinDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t))
	default:
		t.Errorf("unknown receiver type: %s", receiver)
	}
	return sender
}

// ConstructMetricsSender creates a testbed metrics sender from the passed-in metrics sender identifier.
func ConstructMetricsSender(t *testing.T, receiver string) testbed.MetricDataSender {
	var sender testbed.MetricDataSender
	switch receiver {
	case "otlp":
		sender = testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t))
	case "opencensus":
		sender = datasenders.NewOCMetricDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t))
	case "prometheus":
		sender = datasenders.NewPrometheusDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t))
	default:
		t.Errorf("unknown receiver type: %s", receiver)
	}
	return sender
}

// ConstructReceiver creates a testbed receiver from the passed-in recevier identifier.
func ConstructReceiver(t *testing.T, exporter string) testbed.DataReceiver {
	var receiver testbed.DataReceiver
	switch exporter {
	case "otlp":
		receiver = testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t))
	case "opencensus":
		receiver = datareceivers.NewOCDataReceiver(testbed.GetAvailablePort(t))
	case "jaeger":
		receiver = datareceivers.NewJaegerDataReceiver(testbed.GetAvailablePort(t))
	case "zipkin":
		receiver = datareceivers.NewZipkinDataReceiver(testbed.GetAvailablePort(t))
	case "prometheus":
		receiver = datareceivers.NewPrometheusDataReceiver(testbed.GetAvailablePort(t))
	default:
		t.Errorf("unknown exporter type: %s", exporter)
	}
	return receiver
}
