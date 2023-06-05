// Copyright The OpenTelemetry Authors
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
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGetCGroupPathForTask(t *testing.T) {
	cgroupMount := "test"
	controller := "cpu"
	clusterName := "myCluster"

	tests := []struct {
		name    string
		input   string
		wantRes string
		err     error
	}{
		{
			name:    "Task cgroup path exist",
			input:   "test1",
			wantRes: filepath.Join(cgroupMount, controller, "ecs", "test1"),
		},
		{
			name:    "Legacy Task cgroup path exist",
			input:   "test4",
			wantRes: filepath.Join(cgroupMount, controller, "ecs", clusterName, "test4"),
		},
		{
			name:    "CGroup Path does not exist",
			input:   "test5",
			wantRes: "",
			err:     errors.New("CGroup Path " + filepath.Join(cgroupMount, controller, "ecs", clusterName, "test5") + " does not exist"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := getCGroupPathForTask(cgroupMount, controller, tt.input, clusterName)

			if tt.err != nil {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantRes, got)
			}
		})
	}
}

func TestGetCGroupPathFromARN(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantRes string
		err     error
	}{
		{
			name:    "valid old format ARN",
			input:   "arn:aws:ecs:region:aws_account_id:task/task-id",
			wantRes: "task-id",
		},
		{
			name:    "valid new format ARN",
			input:   "arn:aws:ecs:region:aws_account_id:task/cluster-name/task-id2",
			wantRes: "task-id2",
		},
		{
			name:    "invalid format ARN fields are less than 6",
			input:   "arn:aws:tt:aws_account_id:task/task-id",
			wantRes: "",
			err:     errors.New("invalid ecs task arn: arn:aws:tt:aws_account_id:task/task-id"),
		},
		{
			name:    "invalid format ARN with only task",
			input:   "arn:aws:ecs:region:aws_account_id:task",
			wantRes: "",
			err:     errors.New("invalid ecs task arn: arn:aws:ecs:region:aws_account_id:task"),
		},
		{
			name:    "invalid format ARN with more than three split result",
			input:   "arn:aws:ecs:region:aws_account_id:task/region/clustername/task-id/",
			wantRes: "",
			err:     errors.New("invalid ecs task arn: arn:aws:ecs:region:aws_account_id:task/region/clustername/task-id/"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := getTaskCgroupPathFromARN(tt.input)

			if tt.err != nil {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantRes, got)
			}
		})
	}
}

func TestGetCGroupMountPoint(t *testing.T) {

	tests := []struct {
		name    string
		input   string
		wantRes string
		err     error
	}{
		{
			name:    "Get c group mount point successfully",
			input:   "test/mountinfo",
			wantRes: "test",
		},
		{
			name:    "Get c group mount point which is not existed",
			input:   "test/mountinfonotexist",
			wantRes: "",
			err:     errors.New(""),
		},
		{
			name:    "No data",
			input:   "test/mountinfo_err2",
			wantRes: "",
			err:     errors.New(""),
		},
		{
			name:    "we can't detect if the mount is for cgroup",
			input:   "test/mountinfo_err1",
			wantRes: "",
			err:     errors.New(""),
		},
		{
			name:    "the mount is not properly formated",
			input:   "test/mountinfo_err3",
			wantRes: "",
			err:     errors.New(""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := getCGroupMountPoint(tt.input)

			if tt.err != nil {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantRes, got)
			}
		})
	}

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

	tests := []struct {
		name        string
		taskid      string
		clusterName string
		expectRes   int64
	}{
		{
			name:        "Test Get CPU Reserved From Shares",
			taskid:      "test1",
			clusterName: "",
			expectRes:   int64(128),
		},
		{
			name:        "Test Get CPU Reserved From Shares for empty cluster name",
			taskid:      "test4",
			clusterName: "myCluster",
			expectRes:   int64(128),
		},
		{
			name:        "Test Get CPUReserved From Quota",
			taskid:      "test2",
			clusterName: "",
			expectRes:   int64(256),
		},
		{
			name:        "Test Get CPUReserved From Both",
			taskid:      "test3",
			clusterName: "",
			expectRes:   int64(256),
		},
		{
			name:        "Test Get CPUReserved From false Task ID",
			taskid:      "fake",
			clusterName: "",
			expectRes:   int64(0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cgroup.getCPUReservedInTask(tt.taskid, tt.clusterName)
			assert.Equal(t, tt.expectRes, got)
		})
	}

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

	tests := []struct {
		name        string
		taskid      string
		clusterName string
		containers  []ECSContainer
		expectRes   int64
	}{
		{
			name:        "Test Get MEM Reserved From Task without clustername",
			taskid:      "test1",
			clusterName: "",
			containers:  []ECSContainer{},
			expectRes:   int64(256),
		},
		{
			name:        "Test Get MEM Reserved From Task with clustername",
			taskid:      "test3",
			clusterName: "myCluster",
			containers:  []ECSContainer{},
			expectRes:   int64(256),
		},
		{
			name:        "Test Get MEM Reserved From Containers",
			taskid:      "test2",
			clusterName: "",
			containers:  []ECSContainer{{DockerID: "container1"}, {DockerID: "container2"}},
			expectRes:   int64(384),
		},
		{
			name:        "Test Get MEM Reserved From False Task ID",
			taskid:      "fake",
			clusterName: "",
			containers:  []ECSContainer{{DockerID: "container1"}, {DockerID: "container2"}},
			expectRes:   int64(0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cgroup.getMEMReservedInTask(tt.taskid, tt.clusterName, tt.containers)
			assert.Equal(t, tt.expectRes, got)
		})
	}
}

func TestGetCPUReservedAndMemReserved(t *testing.T) {
	var ctx, cancel = context.WithCancel(context.Background())
	var tasks []ECSTask
	var containers []ECSContainer

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
