package datareceivers

import (
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/testbed/testbed"
	"go.uber.org/zap"
)

// AwsXrayReceiver implements AwsXray format receiver.
type AwsXrayReceiver struct {
	testbed.DataReceiverBase
	receiver component.TraceReceiver
}

// NewAwsXrayReceiver creates a new  AwsXrayReceiver
func NewAwsXrayReceiver(port int) *AwsXrayReceiver {
	return &AwsXrayReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

//Start listening on the specified port
func (ar *AwsXrayReceiver) Start(tc *testbed.MockTraceConsumer, mc *testbed.MockMetricConsumer) error {
	var err error
	awsxrayreceiverCfg := awsxrayreceiver.Config{
		Endpoint: fmt.Sprintf("localhost:%d", ar.Port),
		TLSCredentials: &configtls.TLSSetting{
			CertFile: "../../receiver/awsxrayreceiver/server.crt",
			KeyFile:  "../../receiver/awsxrayreceiver/server.key",
		},
	}
	params := component.ReceiverCreateParams{Logger: zap.L()}
	ar.receiver, err = awsxrayreceiver.New(tc, params, &awsxrayreceiverCfg)

	if err != nil {
		return err
	}

	return ar.receiver.Start(context.Background(), ar)
}

func (ar *AwsXrayReceiver) Stop() error {
	ar.receiver.Shutdown(context.Background())
	return nil
}

func (ar *AwsXrayReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(`
  awsxray:
    local_mode: true
    endpoint: localhost:%d
    region: "us-west-2"`, ar.Port)
}

func (ar *AwsXrayReceiver) ProtocolName() string {
	return "awsxray"
}
