// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests/traces"

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers/jaegerdatareceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers/zipkindatareceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders/jaegerdatasender"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders/zipkindatasender"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func constructTraceSender(t *testing.T, name string) testbed.DataSender {
	switch name {
	case "otlp":
		return testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	case "jaeger":
		return jaegerdatasender.NewJaegerGRPCDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	case "zipkin":
		return zipkindatasender.NewZipkinDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	}
	t.Errorf("unknown receiver type: %s", name)
	return nil
}

func constructReceiver(t *testing.T, name string) testbed.DataReceiver {
	switch name {
	case "otlp_grpc":
		return testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
	case "jaeger":
		return jaegerdatareceiver.NewJaegerDataReceiver(testutil.GetAvailablePort(t))
	case "zipkin":
		return zipkindatareceiver.NewZipkinDataReceiver(testutil.GetAvailablePort(t))
	}
	t.Errorf("unknown exporter type: %s", name)
	return nil
}
