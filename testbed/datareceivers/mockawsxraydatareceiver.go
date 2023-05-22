// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datareceivers // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"

import (
	"context"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatareceivers/mockawsxrayreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// MockAwsXrayDataReceiver implements AwsXray format receiver.
type MockAwsXrayDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.Traces
}

// NewMockAwsXrayDataReceiver creates a new  MockDataReceiver
func NewMockAwsXrayDataReceiver(port int) *MockAwsXrayDataReceiver {
	return &MockAwsXrayDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

// Start listening on the specified port
func (ar *MockAwsXrayDataReceiver) Start(tc consumer.Traces, _ consumer.Metrics, _ consumer.Logs) error {
	var err error
	os.Setenv("AWS_ACCESS_KEY_ID", "AWS_ACCESS_KEY_ID")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY")

	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	certs, err := os.ReadFile("../mockdatareceivers/mockawsxrayreceiver/server.crt")

	if err != nil {
		log.Fatalf("Failed to append %q to RootCAs: %v", "../mockdatareceivers/mockawsxrayreceiver/server.crt", err)
	}

	// Append our cert to the system pool
	if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
		log.Println("No certs appended, using system certs only")
	}

	mockDatareceiverCFG := mockawsxrayreceiver.Config{
		Endpoint: fmt.Sprintf("127.0.0.1:%d", ar.Port),
		TLSCredentials: &configtls.TLSSetting{
			CertFile: "../mockdatareceivers/mockawsxrayreceiver/server.crt",
			KeyFile:  "../mockdatareceivers/mockawsxrayreceiver/server.key",
		},
	}
	ar.receiver, err = mockawsxrayreceiver.New(tc, receivertest.NewNopCreateSettings(), &mockDatareceiverCFG)

	if err != nil {
		return err
	}

	return ar.receiver.Start(context.Background(), componenttest.NewNopHost())
}

func (ar *MockAwsXrayDataReceiver) Stop() error {
	return ar.receiver.Shutdown(context.Background())
}

func (ar *MockAwsXrayDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(`
  awsxray:
    local_mode: true
    endpoint: 127.0.0.1:%d
    no_verify_ssl: true
    region: us-west-2`, ar.Port)
}

func (ar *MockAwsXrayDataReceiver) ProtocolName() string {
	return "awsxray"
}
