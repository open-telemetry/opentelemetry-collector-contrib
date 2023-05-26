// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver"

import (
	"context"

	v3c "skywalking.apache.org/repo/goapi/collect/agent/configuration/v3"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	event "skywalking.apache.org/repo/goapi/collect/event/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	profile "skywalking.apache.org/repo/goapi/collect/language/profile/v3"
	management "skywalking.apache.org/repo/goapi/collect/management/v3"
)

type dummyReportService struct {
	management.UnimplementedManagementServiceServer
	v3c.UnimplementedConfigurationDiscoveryServiceServer
	agent.UnimplementedJVMMetricReportServiceServer
	profile.UnimplementedProfileTaskServer
	agent.UnimplementedBrowserPerfServiceServer
	event.UnimplementedEventServiceServer
}

// for sw InstanceProperties
func (d *dummyReportService) ReportInstanceProperties(ctx context.Context, in *management.InstanceProperties) (*common.Commands, error) {
	return &common.Commands{}, nil
}

// for sw InstancePingPkg
func (d *dummyReportService) KeepAlive(ctx context.Context, in *management.InstancePingPkg) (*common.Commands, error) {
	return &common.Commands{}, nil
}

// for sw JVMMetric
func (d *dummyReportService) Collect(_ context.Context, jvm *agent.JVMMetricCollection) (*common.Commands, error) {
	return &common.Commands{}, nil
}

// for sw agent cds
func (d *dummyReportService) FetchConfigurations(_ context.Context, req *v3c.ConfigurationSyncRequest) (*common.Commands, error) {
	return &common.Commands{}, nil
}

// for sw profile
func (d *dummyReportService) GetProfileTaskCommands(_ context.Context, q *profile.ProfileTaskCommandQuery) (*common.Commands, error) {
	return &common.Commands{}, nil
}

func (d *dummyReportService) CollectSnapshot(stream profile.ProfileTask_CollectSnapshotServer) error {
	return nil
}

func (d *dummyReportService) ReportTaskFinish(_ context.Context, report *profile.ProfileTaskFinishReport) (*common.Commands, error) {
	return &common.Commands{}, nil
}
