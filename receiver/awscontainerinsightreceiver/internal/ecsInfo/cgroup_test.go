// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ecsinfo

import (
	"context"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type MockTaskInfo struct {
	tasks            []ECSTask
	runningTaskCount int64
}

func (ii *MockTaskInfo) getRunningTaskCount() int64 {
	return ii.runningTaskCount
}
func (ii *MockTaskInfo) getRunningTasksInfo() []ECSTask {

	return ii.tasks
}

type MockInstanceInfo struct {
	clusterName string
	instanceID  string
}

func (ii *MockInstanceInfo) GetClusterName() string {
	return ii.clusterName
}
func (ii *MockInstanceInfo) GetContainerInstanceID() string {
	return ii.instanceID
}

func TestGetCGroupPathForTask(t *testing.T) {
	cgroupMount := "test"
	controller := "cpu"
	taskID := "test1"
	clusterName := "myCluster"
	result, _ := getCGroupPathForTask(cgroupMount, controller, taskID, clusterName)
	assert.Equal(t, path.Join(cgroupMount, controller, "ecs", taskID), result)

	taskID = "test4"
	result, _ = getCGroupPathForTask(cgroupMount, controller, taskID, clusterName)
	assert.Equal(t, path.Join(cgroupMount, controller, "ecs", clusterName, taskID), result)

	taskID = "test5"
	result, err := getCGroupPathForTask(cgroupMount, controller, taskID, clusterName)
	assert.Equal(t, "", result)
	assert.NotNil(t, err)
}

func TestGetCGroupPathFromARN(t *testing.T) {
	oldFormatARN := "arn:aws:ecs:region:aws_account_id:task/task-id"
	newFormatARN := "arn:aws:ecs:region:aws_account_id:task/cluster-name/task-id"
	result, _ := getTaskCgroupPathFromARN(oldFormatARN)
	assert.Equal(t, "task-id", result, "Expected to be equal")
	result, _ = getTaskCgroupPathFromARN(newFormatARN)
	assert.Equal(t, "task-id", result, "Expected to be equal")
	wrongFormatARN := "arn:aws:ecs:region:aws_account_id:task"
	result, err := getTaskCgroupPathFromARN(wrongFormatARN)
	assert.NotNil(t, err)
	assert.Equal(t, "", result)
	wrongFormatARN2 := "arn:aws:ecs:region:aws_account_id:task/clster-name/taskname/task-id"
	result, err = getTaskCgroupPathFromARN(wrongFormatARN2)
	assert.NotNil(t, err)
	assert.Equal(t, "", result, "Expected to be equal")
}

func TestGetCGroupMountPoint(t *testing.T) {
	result, _ := getCGroupMountPoint("test/mountinfo")
	assert.Equal(t, "test", result, "Expected to be equal")

	_, err := getCGroupMountPoint("test/mountinfonotexist")
	assert.NotNil(t, err)

	_, err = getCGroupMountPoint("test/mountinfo_err1")
	assert.NotNil(t, err)

	_, err = getCGroupMountPoint("test/mountinfo_err2")
	assert.NotNil(t, err)

}

func TestGetCPUReservedInTask(t *testing.T) {
	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	taskinfo := &MockTaskInfo{
		tasks: []ECSTask{},
	}
	containerinfo := &MockInstanceInfo{}

	cgroup := newCGroupScanner(ctx, "test/mountinfo", zap.NewNop(), taskinfo, containerinfo, time.Minute)
	ctx.Done()
	//TestGetCPUReservedFromShares

	assert.Equal(t, int64(128), cgroup.getCPUReservedInTask("test1", ""), "TestGetCPUReservedFromShares expected to be equal")

	assert.Equal(t, int64(128), cgroup.getCPUReservedInTask("test4", "myCluster"), "TestGetCPUReservedFromShares for empty cluster name expected to be equal")

	//TestGetCPUReservedFromQuota
	assert.Equal(t, int64(256), cgroup.getCPUReservedInTask("test2", ""), "TestGetCPUReservedFromQuota expected to be equal")

	//TestGetCPUReservedFromBoth
	assert.Equal(t, int64(256), cgroup.getCPUReservedInTask("test3", ""), "TestGetCPUReservedFromBoth expected to be equal")

	//TestGetCPUReservedFromFalseTaskID
	assert.Equal(t, int64(0), cgroup.getCPUReservedInTask("fake", ""), "TestGetCPUReservedFromFalseTaskID expected to be equal")

}

func TestGetMEMReservedInTask(t *testing.T) {
	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	taskinfo := &MockTaskInfo{
		tasks: []ECSTask{},
	}
	containerinfo := &MockInstanceInfo{}
	cgroup := newCGroupScanner(ctx, "test/mountinfo", zap.NewNop(), taskinfo, containerinfo, time.Minute)
	ctx.Done()
	//TestGetMEMReservedFromTask
	containers := []ECSContainer{}
	assert.Equal(t, int64(256), cgroup.getMEMReservedInTask("test1", "", containers), "TestGetMEMReservedFromTask expected to be equal")
	assert.Equal(t, int64(256), cgroup.getMEMReservedInTask("test3", "myCluster", containers), "TestGetMEMReservedFromTask for exmpty cluster name expected to be equal")

	//TestGetMEMReservedFromContainers
	containers = []ECSContainer{{DockerID: "container1"}, {DockerID: "container2"}}
	assert.Equal(t, int64(384), cgroup.getMEMReservedInTask("test2", "", containers), "TestGetMEMReservedFromContainers expected to be equal")

	//TestGetMEMReservedFromFalseTaskID
	containers = []ECSContainer{{DockerID: "container1"}, {DockerID: "container2"}}
	assert.Equal(t, int64(0), cgroup.getMEMReservedInTask("fake", "", containers), "TestGetMEMReservedFromFalseTaskID expected to be equal")

}

func TestGetCPUReservedAndMemReserved(t *testing.T) {
	var ctx, cancel = context.WithCancel(context.Background())
	tasks := []ECSTask{}
	containers := []ECSContainer{}

	task1 := ECSTask{
		KnownStatus: "RUNNING",
		ARN:         "arn:aws:ecs:region:aws_account_id:task/myCluster/test1",
		Containers:  containers,
	}

	task3 := ECSTask{
		KnownStatus: "RUNNING",
		ARN:         "arn:aws:ecs:region:aws_account_id:task/myCluster/test3",
		Containers:  containers,
	}

	container1 := ECSContainer{
		DockerID: "container1",
	}
	container2 := ECSContainer{
		DockerID: "container2",
	}

	containers = append(containers, container1)
	containers = append(containers, container2)

	task2 := ECSTask{
		KnownStatus: "RUNNING",
		ARN:         "arn:aws:ecs:region:aws_account_id:task/myCluster/test2",
		Containers:  containers,
	}

	tasks = append(tasks, task1)
	tasks = append(tasks, task2)
	tasks = append(tasks, task3)

	taskinfo := &MockTaskInfo{
		tasks: tasks,
	}

	containerinfo := &MockInstanceInfo{
		clusterName: "myCluster",
	}

	cgroup := newCGroupScanner(ctx, "test/mountinfo", zap.NewNop(), taskinfo, containerinfo, time.Minute)

	assert.NotNil(t, cgroup)

	assert.Equal(t, int64(640), cgroup.getCPUReserved())

	assert.Equal(t, int64(896), cgroup.getMemReserved())

	cancel()
	// test err
	cgroup = newCGroupScannerForContainer(ctx, zap.NewNop(), taskinfo, containerinfo, time.Minute)

	assert.NotNil(t, cgroup)

	assert.Equal(t, int64(0), cgroup.getCPUReserved())

	assert.Equal(t, int64(0), cgroup.getMemReserved())

}
