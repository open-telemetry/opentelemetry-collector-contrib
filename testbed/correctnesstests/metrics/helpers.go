// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests/metrics"

import (
	"testing"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers/stefdatareceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders/stefdatasender"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func newDbgLogger() *zap.Logger {
	logger, err := zap.NewDevelopment(zap.Fields(zap.String("type", "testbed")))
	if err != nil {
		panic("Cannot create logger " + err.Error())
	}
	return logger
}

func constructMetricsSender(t *testing.T, name string) testbed.MetricDataSender {
	switch name {
	case "otlp":
		return testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	case "stef":
		s := stefdatasender.NewStefDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
		s.Logger = newDbgLogger()
		return s
	}
	t.Errorf("unknown receiver type: %s", name)
	return nil
}

func constructReceiver(t *testing.T, name string) testbed.DataReceiver {
	switch name {
	case "otlp_grpc":
		return testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
	case "stef":
		r := stefdatareceiver.NewStefDataReceiver(testutil.GetAvailablePort(t))
		r.Logger = newDbgLogger()
		return r
	}
	t.Errorf("unknown exporter type: %s", name)
	return nil
}
