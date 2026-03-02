// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"

type MockECSInfo struct {
	ClusterName string
	InstanceIP  string
}

func (*MockECSInfo) GetRunningTaskCount() int64 {
	return 2
}

func (*MockECSInfo) GetCPUReserved() int64 {
	return 32
}

func (*MockECSInfo) GetMemReserved() int64 {
	return 213
}

func (*MockECSInfo) GetContainerInstanceID() string {
	return "eeee12.dsfr"
}

func (*MockECSInfo) GetClusterName() string {
	return "ecs-cluster"
}
