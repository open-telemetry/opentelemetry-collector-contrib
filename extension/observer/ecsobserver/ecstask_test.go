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
	"log"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
)

func buildWorkloadFargateAwsvpc(dockerLabel bool, taskDef bool) *ECSTask {
	networkMode := ecs.NetworkModeAwsvpc
	taskAttachmentId := "775c6c63-b5f7-4a5b-8a60-8f8295a04cda"
	taskAttachmentType := "ElasticNetworkInterface"
	taskAttachmentStatus := "ATTACHING"
	taskAttachmentDetailsKey1 := "networkInterfaceId"
	taskAttachmentDetailsKey2 := "privateIPv4Address"
	taskAttachmentDetailsValue1 := "eni-03de9d47faaa2e5ec"
	taskAttachmentDetailsValue2 := "10.0.0.129"

	taskDefinitionArn := "arn:aws:ecs:us-east-2:211220956907:task-definition/prometheus-java-tomcat-fargate-awsvpc:1"
	var taskRevision int64 = 4
	port9404String := "9404"
	port9406String := "9406"
	var port9404Int64 int64 = 9404
	var port9406Int64 int64 = 9406
	containerNameTomcat := "bugbash-tomcat-fargate-awsvpc-with-docker-label"
	containerNameJar := "bugbash-jar-fargate-awsvpc-with-dockerlabel"

	jobNameLabel := "java-tomcat-fargate-awsvpc"
	metricsPathLabel := "/metrics"

	return &ECSTask{
		DockerLabelBased:    dockerLabel,
		TaskDefinitionBased: taskDef,
		Task: &ecs.Task{
			TaskDefinitionArn: &taskDefinitionArn,
			Attachments: []*ecs.Attachment{
				{
					Id:     &taskAttachmentId,
					Type:   &taskAttachmentType,
					Status: &taskAttachmentStatus,
					Details: []*ecs.KeyValuePair{
						{
							Name:  &taskAttachmentDetailsKey1,
							Value: &taskAttachmentDetailsValue1,
						},
						{
							Name:  &taskAttachmentDetailsKey2,
							Value: &taskAttachmentDetailsValue2,
						},
					},
				},
			},
		},
		TaskDefinition: &ecs.TaskDefinition{
			NetworkMode:       &networkMode,
			TaskDefinitionArn: &taskDefinitionArn,
			Revision:          &taskRevision,
			ContainerDefinitions: []*ecs.ContainerDefinition{
				{
					Name: &containerNameTomcat,
					DockerLabels: map[string]*string{
						"FARGATE_PROMETHEUS_EXPORTER_PORT": &port9404String,
						"FARGATE_PROMETHEUS_JOB_NAME":      &jobNameLabel,
					},
					PortMappings: []*ecs.PortMapping{
						{
							ContainerPort: &port9404Int64,
							HostPort:      &port9404Int64,
						},
					},
				},
				{
					Name: &containerNameJar,
					DockerLabels: map[string]*string{
						"FARGATE_PROMETHEUS_EXPORTER_PORT": &port9406String,
						"ECS_PROMETHEUS_METRICS_PATH":      &metricsPathLabel,
					},
					PortMappings: []*ecs.PortMapping{
						{
							ContainerPort: &port9406Int64,
							HostPort:      &port9406Int64,
						},
					},
				},
			},
		},
	}
}

func buildWorkloadEC2BridgeDynamicPort(dockerLabel bool, taskDef bool) *ECSTask {
	networkMode := ecs.NetworkModeBridge
	taskContainersArn := "arn:aws:ecs:us-east-2:211220956907:container/3b288961-eb2c-4de5-a4c5-682c0a7cc625"
	var taskContainersDynamicHostPort int64 = 32774
	var taskContainersMappedHostPort int64 = 9494

	taskDefinitionArn := "arn:aws:ecs:us-east-2:211220956907:task-definition/prometheus-java-tomcat-ec2-awsvpc:1"
	var taskRevision int64 = 5
	port9404String := "9404"
	port9406String := "9406"
	var port9404Int64 int64 = 9404
	var port9406Int64 int64 = 9406
	var port0Int64 int64 = 0

	containerNameTomcat := "bugbash-tomcat-prometheus-workload-java-ec2-bridge-mapped-port"
	containerNameJar := "bugbash-jar-prometheus-workload-java-ec2-bridge"

	jobNameLabelTomcat := "bugbash-tomcat-ec2-bridge-mapped-port"
	metricsPathLabel := "/metrics"

	return &ECSTask{
		DockerLabelBased:    dockerLabel,
		TaskDefinitionBased: taskDef,
		EC2Info: &EC2MetaData{
			ContainerInstanceId: "arn:aws:ecs:us-east-2:211220956907:container-instance/7b0a9662-ee0b-4cf6-9391-03f50ca501a5",
			ECInstanceId:        "i-02aa8e82e91b2c30e",
			PrivateIP:           "10.4.0.205",
			InstanceType:        "t3.medium",
			VpcId:               "vpc-03e9f55a92516a5e4",
			SubnetId:            "subnet-0d0b0212d14b70250",
		},
		Task: &ecs.Task{
			TaskDefinitionArn: &taskDefinitionArn,
			Attachments:       []*ecs.Attachment{},
			Containers: []*ecs.Container{
				{
					ContainerArn: &taskContainersArn,
					Name:         &containerNameTomcat,
					NetworkBindings: []*ecs.NetworkBinding{
						{
							ContainerPort: &port9404Int64,
							HostPort:      &taskContainersMappedHostPort,
						},
					},
				},
				{
					ContainerArn: &taskContainersArn,
					Name:         &containerNameJar,
					NetworkBindings: []*ecs.NetworkBinding{
						{
							ContainerPort: &port9404Int64,
							HostPort:      &taskContainersDynamicHostPort,
						},
					},
				},
			},
		},
		TaskDefinition: &ecs.TaskDefinition{
			NetworkMode:       &networkMode,
			TaskDefinitionArn: &taskDefinitionArn,
			Revision:          &taskRevision,
			ContainerDefinitions: []*ecs.ContainerDefinition{
				{
					Name: &containerNameTomcat,
					DockerLabels: map[string]*string{
						"EC2_PROMETHEUS_EXPORTER_PORT": &port9404String,
						"EC2_PROMETHEUS_JOB_NAME":      &jobNameLabelTomcat,
					},
					PortMappings: []*ecs.PortMapping{
						{
							ContainerPort: &port9404Int64,
							HostPort:      &port9404Int64,
						},
					},
				},
				{
					Name: &containerNameJar,
					DockerLabels: map[string]*string{
						"EC2_PROMETHEUS_EXPORTER_PORT": &port9406String,
						"EC2_PROMETHEUS_METRICS_PATH":  &metricsPathLabel,
					},
					PortMappings: []*ecs.PortMapping{
						{
							ContainerPort: &port9406Int64,
							HostPort:      &port0Int64,
						},
					},
				},
			},
		},
	}
}

func TestAddPrometheusTargets_DockerLabelBased(t *testing.T) {
	fullTask := buildWorkloadFargateAwsvpc(true, false)
	assert.Equal(t, "10.0.0.129", fullTask.getPrivateIp())

	config := &Config{
		DockerLabel: &DockerLabelConfig{
			JobNameLabel:     "FARGATE_PROMETHEUS_JOB_NAME",
			PortLabel:        "FARGATE_PROMETHEUS_EXPORTER_PORT",
			MetricsPathLabel: "ECS_PROMETHEUS_METRICS_PATH",
		},
	}

	targets := make(map[string]*PrometheusTarget)
	fullTask.addPrometheusTargets(targets, config)

	assert.Equal(t, 2, len(targets))
	target, ok := targets["10.0.0.129:9404/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9404/metrics")

	assert.Equal(t, 5, len(target.Labels))
	assert.Equal(t, "java-tomcat-fargate-awsvpc", target.Labels["job"])
	assert.Equal(t, "bugbash-tomcat-fargate-awsvpc-with-docker-label", target.Labels["container_name"])
	assert.Equal(t, "4", target.Labels["TaskRevision"])
	assert.Equal(t, "9404", target.Labels["FARGATE_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "java-tomcat-fargate-awsvpc", target.Labels["FARGATE_PROMETHEUS_JOB_NAME"])

	target, ok = targets["10.0.0.129:9406/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9406/metrics")
	assert.Equal(t, 5, len(target.Labels))
	assert.Equal(t, "4", target.Labels["TaskRevision"])
	assert.Equal(t, "bugbash-jar-fargate-awsvpc-with-dockerlabel", target.Labels["container_name"])
	assert.Equal(t, "9406", target.Labels["FARGATE_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "/metrics", target.Labels["__metrics_path__"])
	assert.Equal(t, "/metrics", target.Labels["ECS_PROMETHEUS_METRICS_PATH"])
}

func TestAddPrometheusTargets_TaskDefBased_Fargate(t *testing.T) {
	fullTask := buildWorkloadFargateAwsvpc(false, true)
	assert.Equal(t, "10.0.0.129", fullTask.getPrivateIp())
	config := &Config{
		TaskDefinitions: []*TaskDefinitionConfig{
			{
				JobName:           "",
				MetricsPorts:      "9404;9406",
				TaskDefArnPattern: ".*:task-definition/prometheus-java-tomcat-fargate-awsvpc:[0-9]+",
				MetricsPath:       "/stats/metrics",
			},
		},
	}
	config.TaskDefinitions[0].init()
	assert.Equal(t, []int{9404, 9406}, config.TaskDefinitions[0].metricsPortList)

	targets := make(map[string]*PrometheusTarget)
	fullTask.addPrometheusTargets(targets, config)

	assert.Equal(t, 2, len(targets))
	target, ok := targets["10.0.0.129:9404/stats/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9404/stats/metrics")

	assert.Equal(t, 5, len(target.Labels))
	assert.Equal(t, "java-tomcat-fargate-awsvpc", target.Labels["FARGATE_PROMETHEUS_JOB_NAME"])
	assert.Equal(t, "bugbash-tomcat-fargate-awsvpc-with-docker-label", target.Labels["container_name"])
	assert.Equal(t, "4", target.Labels["TaskRevision"])
	assert.Equal(t, "9404", target.Labels["FARGATE_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "/stats/metrics", target.Labels["__metrics_path__"])

	target, ok = targets["10.0.0.129:9406/stats/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9406/stats/metrics")
	assert.Equal(t, 5, len(target.Labels))
	assert.Equal(t, "4", target.Labels["TaskRevision"])
	assert.Equal(t, "bugbash-jar-fargate-awsvpc-with-dockerlabel", target.Labels["container_name"])
	assert.Equal(t, "9406", target.Labels["FARGATE_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "/stats/metrics", target.Labels["__metrics_path__"])
	assert.Equal(t, "/metrics", target.Labels["ECS_PROMETHEUS_METRICS_PATH"])
}

func TestAddPrometheusTargets_TaskDefBased_EC2(t *testing.T) {
	fullTask := buildWorkloadEC2BridgeDynamicPort(false, true)
	assert.Equal(t, "10.4.0.205", fullTask.getPrivateIp())
	config := &Config{
		TaskDefinitions: []*TaskDefinitionConfig{
			{
				JobName:              "",
				MetricsPorts:         "9404;9406",
				TaskDefArnPattern:    ".*:task-definition/prometheus-java-tomcat-ec2-awsvpc:[0-9]+",
				MetricsPath:          "/metrics",
				ContainerNamePattern: ".*tomcat-prometheus-workload-java-ec2.*",
			},
		},
	}
	config.TaskDefinitions[0].init()
	assert.Equal(t, []int{9404, 9406}, config.TaskDefinitions[0].metricsPortList)

	targets := make(map[string]*PrometheusTarget)
	fullTask.addPrometheusTargets(targets, config)

	assert.Equal(t, 1, len(targets))
	target, ok := targets["10.4.0.205:9494/metrics"]
	log.Print(target)
	assert.True(t, ok, "Missing target: 10.4.0.205:9494/metrics")
	assert.Equal(t, 8, len(target.Labels))
	assert.Equal(t, "9404", target.Labels["EC2_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "bugbash-tomcat-ec2-bridge-mapped-port", target.Labels["EC2_PROMETHEUS_JOB_NAME"])
	assert.Equal(t, "t3.medium", target.Labels["InstanceType"])
	assert.Equal(t, "subnet-0d0b0212d14b70250", target.Labels["SubnetId"])
	assert.Equal(t, "5", target.Labels["TaskRevision"])
	assert.Equal(t, "vpc-03e9f55a92516a5e4", target.Labels["VpcId"])
	assert.Equal(t, "/metrics", target.Labels["__metrics_path__"])
	assert.Equal(t, "bugbash-tomcat-prometheus-workload-java-ec2-bridge-mapped-port", target.Labels["container_name"])
}

func TestAddPrometheusTargets_Mixed_Fargate(t *testing.T) {
	fullTask := buildWorkloadFargateAwsvpc(true, true)
	log.Print(fullTask)
	assert.Equal(t, "10.0.0.129", fullTask.getPrivateIp())
	config := &Config{
		DockerLabel: &DockerLabelConfig{
			JobNameLabel:     "FARGATE_PROMETHEUS_JOB_NAME",
			PortLabel:        "FARGATE_PROMETHEUS_EXPORTER_PORT",
			MetricsPathLabel: "ECS_PROMETHEUS_METRICS_PATH",
		},
		TaskDefinitions: []*TaskDefinitionConfig{
			{
				JobName:           "",
				MetricsPorts:      "9404;9406",
				TaskDefArnPattern: ".*:task-definition/prometheus-java-tomcat-fargate-awsvpc:[0-9]+",
				MetricsPath:       "/stats/metrics",
			},
		},
	}
	config.TaskDefinitions[0].init()
	assert.Equal(t, []int{9404, 9406}, config.TaskDefinitions[0].metricsPortList)

	targets := make(map[string]*PrometheusTarget)
	fullTask.addPrometheusTargets(targets, config)
	assert.Equal(t, 4, len(targets))
	_, ok := targets["10.0.0.129:9404/stats/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9404/stats/metrics")
	_, ok = targets["10.0.0.129:9406/stats/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9406/stats/metrics")
	_, ok = targets["10.0.0.129:9404/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9404/metrics")
	_, ok = targets["10.0.0.129:9406/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9406/metrics")
}

func TestAddPrometheusTargets_Mixed_EC2(t *testing.T) {
	fullTask := buildWorkloadEC2BridgeDynamicPort(true, true)
	assert.Equal(t, "10.4.0.205", fullTask.getPrivateIp())
	config := &Config{
		DockerLabel: &DockerLabelConfig{
			JobNameLabel:     "EC2_PROMETHEUS_JOB_NAME",
			PortLabel:        "EC2_PROMETHEUS_EXPORTER_PORT",
			MetricsPathLabel: "ECS_PROMETHEUS_METRICS_PATH",
		},
		TaskDefinitions: []*TaskDefinitionConfig{
			{
				JobName:           "",
				MetricsPorts:      "9404;9406",
				TaskDefArnPattern: ".*:task-definition/prometheus-java-tomcat-ec2-awsvpc:[0-9]+",
				MetricsPath:       "/metrics",
			},
		},
	}
	config.TaskDefinitions[0].init()
	assert.Equal(t, []int{9404, 9406}, config.TaskDefinitions[0].metricsPortList)

	targets := make(map[string]*PrometheusTarget)
	fullTask.addPrometheusTargets(targets, config)

	assert.Equal(t, 2, len(targets))
	target, ok := targets["10.4.0.205:32774/metrics"]
	assert.True(t, ok, "Missing target: 10.4.0.205:32774/metrics")

	assert.Equal(t, 8, len(target.Labels))
	assert.Equal(t, "/metrics", target.Labels["EC2_PROMETHEUS_METRICS_PATH"])
	assert.Equal(t, "9406", target.Labels["EC2_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "t3.medium", target.Labels["InstanceType"])
	assert.Equal(t, "subnet-0d0b0212d14b70250", target.Labels["SubnetId"])
	assert.Equal(t, "5", target.Labels["TaskRevision"])
	assert.Equal(t, "vpc-03e9f55a92516a5e4", target.Labels["VpcId"])
	assert.Equal(t, "/metrics", target.Labels["__metrics_path__"])
	assert.Equal(t, "bugbash-jar-prometheus-workload-java-ec2-bridge", target.Labels["container_name"])

	target, ok = targets["10.4.0.205:9494/metrics"]
	assert.True(t, ok, "Missing target: 10.4.0.205:9494/metrics")
	assert.Equal(t, 8, len(target.Labels))
	assert.Equal(t, "9404", target.Labels["EC2_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "bugbash-tomcat-ec2-bridge-mapped-port", target.Labels["EC2_PROMETHEUS_JOB_NAME"])
	assert.Equal(t, "t3.medium", target.Labels["InstanceType"])
	assert.Equal(t, "subnet-0d0b0212d14b70250", target.Labels["SubnetId"])
	assert.Equal(t, "5", target.Labels["TaskRevision"])
	assert.Equal(t, "vpc-03e9f55a92516a5e4", target.Labels["VpcId"])
	assert.Equal(t, "bugbash-tomcat-prometheus-workload-java-ec2-bridge-mapped-port", target.Labels["container_name"])
	assert.Equal(t, "bugbash-tomcat-ec2-bridge-mapped-port", target.Labels["job"])
}

func TestAddTargetLabel(t *testing.T) {
	labels := make(map[string]string)
	var labelValueB string
	labelValueC := "label_value_c"
	addTargetLabel(labels, "Key_A", nil)
	addTargetLabel(labels, "Key_B", &labelValueB)
	addTargetLabel(labels, "Key_C", &labelValueC)
	expected := map[string]string{"Key_C": "label_value_c"}
	assert.True(t, reflect.DeepEqual(labels, expected))
}
