// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"

type MockECSInfo struct {
	ClusterName string
	InstanceIP  string
}

func (e *MockECSInfo) GetRunningTaskCount() int64 {
	return 2
}

func (e *MockECSInfo) GetCPUReserved() int64 {
	return 32
}

func (e *MockECSInfo) GetMemReserved() int64 {
	return 213
}

func (e *MockECSInfo) GetContainerInstanceID() string {
	return "eeee12.dsfr"
}

func (e *MockECSInfo) GetClusterName() string {
	return "ecs-cluster"
}
