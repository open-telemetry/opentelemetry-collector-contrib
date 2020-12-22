// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ecsobserver

import (
	"github.com/aws/aws-sdk-go/service/ecs"
)

const (
	containerNameLabel   = "container_name"
	taskFamilyLabel      = "TaskDefinitionFamily"
	taskRevisionLabel    = "TaskRevision"
	taskGroupLabel       = "TaskGroup"
	taskStartedbyLabel   = "StartedBy"
	taskLaunchTypeLabel  = "LaunchType"
	taskJobNameLabel     = "job"
	taskMetricsPathLabel = "__metrics_path__"
	ec2InstanceTypeLabel = "InstanceType"
	ec2VpcIdLabel        = "VpcId"
	ec2SubnetIdLabel     = "SubnetId"

	//https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
	defaultPrometheusMetricsPath = "/metrics"
)

type EC2MetaData struct {
	ContainerInstanceId string
	ECInstanceId        string
	PrivateIP           string
	InstanceType        string
	VpcId               string
	SubnetId            string
}

type ECSTask struct {
	Task           *ecs.Task
	TaskDefinition *ecs.TaskDefinition
	EC2Info        *EC2MetaData

	DockerLabelBased    bool
	TaskDefinitionBased bool
}
