// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver"

import (
	"context"

	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

type clrService struct {
	agent.UnimplementedCLRMetricReportServiceServer
}

func (c *clrService) Collect(ctx context.Context, req *agent.CLRMetricCollection) (*common.Commands, error) {
	return &common.Commands{}, nil
}
