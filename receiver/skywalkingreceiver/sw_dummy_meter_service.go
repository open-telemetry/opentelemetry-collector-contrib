// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver"

import (
	meter "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

type meterService struct {
	meter.UnimplementedMeterReportServiceServer
}

func (m *meterService) Collect(stream meter.MeterReportService_CollectServer) error {
	return nil
}

func (m *meterService) CollectBatch(batch meter.MeterReportService_CollectBatchServer) error {
	return nil
}
