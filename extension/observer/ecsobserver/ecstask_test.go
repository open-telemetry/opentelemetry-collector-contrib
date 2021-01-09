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
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func buildWorkloadFargateAwsvpc(dockerLabel bool, taskDef bool) *ECSTask {
	networkMode := ecs.NetworkModeAwsvpc
	taskAttachmentID := "775c6c63-b5f7-4a5b-8a60-8f8295a04cda"
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
	containerNameJar := "bugbash-jar-fargate-awsvpc-with-docker-label"

	jobNameLabel := "java-tomcat-fargate-awsvpc"
	metricsPathLabel := "/metrics"

	return &ECSTask{
		DockerLabelBased:    dockerLabel,
		TaskDefinitionBased: taskDef,
		Task: &ecs.Task{
			TaskDefinitionArn: &taskDefinitionArn,
			Attachments: []*ecs.Attachment{
				{
					Id:     &taskAttachmentID,
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
			ContainerInstanceID: "arn:aws:ecs:us-east-2:211220956907:container-instance/7b0a9662-ee0b-4cf6-9391-03f50ca501a5",
			ECInstanceID:        "i-02aa8e82e91b2c30e",
			PrivateIP:           "10.4.0.205",
			InstanceType:        "t3.medium",
			VpcID:               "vpc-03e9f55a92516a5e4",
			SubnetID:            "subnet-0d0b0212d14b70250",
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

func TestAddTargets(t *testing.T) {
	testCases := []struct {
		testName        string
		task            *ECSTask
		config          *Config
		expectedTargets map[string]*Target
	}{
		{
			"docker label-based",
			buildWorkloadFargateAwsvpc(true, false),
			&Config{
				DockerLabel: &DockerLabelConfig{
					JobNameLabel:     "FARGATE_PROMETHEUS_JOB_NAME",
					PortLabel:        "FARGATE_PROMETHEUS_EXPORTER_PORT",
					MetricsPathLabel: "ECS_PROMETHEUS_METRICS_PATH",
				},
			},
			map[string]*Target{
				"10.0.0.129:9404": {
					IP:          "10.0.0.129",
					Port:        9404,
					MetricsPath: "",
					Labels: map[string]string{
						"job":                              "java-tomcat-fargate-awsvpc",
						"container_name":                   "bugbash-tomcat-fargate-awsvpc-with-docker-label",
						"TaskRevision":                     "4",
						"FARGATE_PROMETHEUS_EXPORTER_PORT": "9404",
						"FARGATE_PROMETHEUS_JOB_NAME":      "java-tomcat-fargate-awsvpc",
					},
				},
				"10.0.0.129:9406/metrics": {
					IP:          "10.0.0.129",
					Port:        9406,
					MetricsPath: "/metrics",
					Labels: map[string]string{
						"container_name":                   "bugbash-jar-fargate-awsvpc-with-docker-label",
						"TaskRevision":                     "4",
						"FARGATE_PROMETHEUS_EXPORTER_PORT": "9406",
						"__metrics_path__":                 "/metrics",
						"ECS_PROMETHEUS_METRICS_PATH":      "/metrics",
					},
				},
			},
		},
		{
			"task definition-based, fargate",
			buildWorkloadFargateAwsvpc(false, true),
			&Config{
				TaskDefinitions: []*TaskDefinitionConfig{
					{
						JobName:           "",
						MetricsPorts:      "9404;9406",
						TaskDefArnPattern: ".*:task-definition/prometheus-java-tomcat-fargate-awsvpc:[0-9]+",
						MetricsPath:       "/stats/metrics",
					},
				},
			},
			map[string]*Target{
				"10.0.0.129:9404/stats/metrics": {
					IP:          "10.0.0.129",
					Port:        9404,
					MetricsPath: "/stats/metrics",
					Labels: map[string]string{
						"container_name":                   "bugbash-tomcat-fargate-awsvpc-with-docker-label",
						"TaskRevision":                     "4",
						"FARGATE_PROMETHEUS_EXPORTER_PORT": "9404",
						"__metrics_path__":                 "/stats/metrics",
						"FARGATE_PROMETHEUS_JOB_NAME":      "java-tomcat-fargate-awsvpc",
					},
				},
				"10.0.0.129:9406/stats/metrics": {
					IP:          "10.0.0.129",
					Port:        9406,
					MetricsPath: "/stats/metrics",
					Labels: map[string]string{
						"container_name":                   "bugbash-jar-fargate-awsvpc-with-docker-label",
						"TaskRevision":                     "4",
						"FARGATE_PROMETHEUS_EXPORTER_PORT": "9406",
						"__metrics_path__":                 "/stats/metrics",
						"ECS_PROMETHEUS_METRICS_PATH":      "/metrics",
					},
				},
			},
		},
		{
			"task definition-based, ec2",
			buildWorkloadEC2BridgeDynamicPort(false, true),
			&Config{
				TaskDefinitions: []*TaskDefinitionConfig{
					{
						JobName:              "",
						MetricsPorts:         "9404;9406",
						TaskDefArnPattern:    ".*:task-definition/prometheus-java-tomcat-ec2-awsvpc:[0-9]+",
						MetricsPath:          "/metrics",
						ContainerNamePattern: ".*tomcat-prometheus-workload-java-ec2.*",
					},
				},
			},
			map[string]*Target{
				"10.4.0.205:9494/metrics": {
					IP:          "10.4.0.205",
					Port:        9494,
					MetricsPath: "/metrics",
					Labels: map[string]string{
						"container_name":               "bugbash-tomcat-prometheus-workload-java-ec2-bridge-mapped-port",
						"TaskRevision":                 "5",
						"VpcId":                        "vpc-03e9f55a92516a5e4",
						"EC2_PROMETHEUS_EXPORTER_PORT": "9404",
						"__metrics_path__":             "/metrics",
						"EC2_PROMETHEUS_JOB_NAME":      "bugbash-tomcat-ec2-bridge-mapped-port",
						"InstanceType":                 "t3.medium",
						"SubnetId":                     "subnet-0d0b0212d14b70250",
					},
				},
			},
		},
		{
			"mixed targets, fargate",
			buildWorkloadFargateAwsvpc(true, true),
			&Config{
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
			},
			map[string]*Target{
				"10.0.0.129:9404": {
					IP:          "10.0.0.129",
					Port:        9404,
					MetricsPath: "",
					Labels: map[string]string{
						"job":                              "java-tomcat-fargate-awsvpc",
						"container_name":                   "bugbash-tomcat-fargate-awsvpc-with-docker-label",
						"TaskRevision":                     "4",
						"FARGATE_PROMETHEUS_EXPORTER_PORT": "9404",
						"FARGATE_PROMETHEUS_JOB_NAME":      "java-tomcat-fargate-awsvpc",
					},
				},
				"10.0.0.129:9406/metrics": {
					IP:          "10.0.0.129",
					Port:        9406,
					MetricsPath: "/metrics",
					Labels: map[string]string{
						"container_name":                   "bugbash-jar-fargate-awsvpc-with-docker-label",
						"TaskRevision":                     "4",
						"FARGATE_PROMETHEUS_EXPORTER_PORT": "9406",
						"__metrics_path__":                 "/metrics",
						"ECS_PROMETHEUS_METRICS_PATH":      "/metrics",
					},
				},
				"10.0.0.129:9404/stats/metrics": {
					IP:          "10.0.0.129",
					Port:        9404,
					MetricsPath: "/stats/metrics",
					Labels: map[string]string{
						"container_name":                   "bugbash-tomcat-fargate-awsvpc-with-docker-label",
						"TaskRevision":                     "4",
						"FARGATE_PROMETHEUS_EXPORTER_PORT": "9404",
						"__metrics_path__":                 "/stats/metrics",
						"FARGATE_PROMETHEUS_JOB_NAME":      "java-tomcat-fargate-awsvpc",
					},
				},
				"10.0.0.129:9406/stats/metrics": {
					IP:          "10.0.0.129",
					Port:        9406,
					MetricsPath: "/stats/metrics",
					Labels: map[string]string{
						"container_name":                   "bugbash-jar-fargate-awsvpc-with-docker-label",
						"TaskRevision":                     "4",
						"FARGATE_PROMETHEUS_EXPORTER_PORT": "9406",
						"__metrics_path__":                 "/stats/metrics",
						"ECS_PROMETHEUS_METRICS_PATH":      "/metrics",
					},
				},
			},
		},
		{
			"mixed targets, ec2",
			buildWorkloadEC2BridgeDynamicPort(true, true),
			&Config{
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
			},
			map[string]*Target{
				"10.4.0.205:32774/metrics": {
					IP:          "10.4.0.205",
					Port:        32774,
					MetricsPath: "/metrics",
					Labels: map[string]string{
						"InstanceType":                 "t3.medium",
						"SubnetId":                     "subnet-0d0b0212d14b70250",
						"VpcId":                        "vpc-03e9f55a92516a5e4",
						"container_name":               "bugbash-jar-prometheus-workload-java-ec2-bridge",
						"TaskRevision":                 "5",
						"EC2_PROMETHEUS_EXPORTER_PORT": "9406",
						"EC2_PROMETHEUS_METRICS_PATH":  "/metrics",
						"__metrics_path__":             "/metrics",
					},
				},
				"10.4.0.205:9494": {
					IP:          "10.4.0.205",
					Port:        9494,
					MetricsPath: "",
					Labels: map[string]string{
						"job":                          "bugbash-tomcat-ec2-bridge-mapped-port",
						"InstanceType":                 "t3.medium",
						"SubnetId":                     "subnet-0d0b0212d14b70250",
						"VpcId":                        "vpc-03e9f55a92516a5e4",
						"container_name":               "bugbash-tomcat-prometheus-workload-java-ec2-bridge-mapped-port",
						"TaskRevision":                 "5",
						"EC2_PROMETHEUS_JOB_NAME":      "bugbash-tomcat-ec2-bridge-mapped-port",
						"EC2_PROMETHEUS_EXPORTER_PORT": "9404",
					},
				},
				"10.4.0.205:9494/metrics": {
					IP:          "10.4.0.205",
					Port:        9494,
					MetricsPath: "/metrics",
					Labels: map[string]string{
						"InstanceType":                 "t3.medium",
						"SubnetId":                     "subnet-0d0b0212d14b70250",
						"VpcId":                        "vpc-03e9f55a92516a5e4",
						"container_name":               "bugbash-tomcat-prometheus-workload-java-ec2-bridge-mapped-port",
						"TaskRevision":                 "5",
						"EC2_PROMETHEUS_JOB_NAME":      "bugbash-tomcat-ec2-bridge-mapped-port",
						"EC2_PROMETHEUS_EXPORTER_PORT": "9404",
						"__metrics_path__":             "/metrics",
					},
				},
			},
		},
		{
			"no network mode",
			&ECSTask{
				TaskDefinition: &ecs.TaskDefinition{},
			},
			&Config{
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
			},
			map[string]*Target{},
		},
	}

	for _, tc := range testCases {
		tc.config.logger = zap.NewNop()
		t.Run(tc.testName, func(t *testing.T) {
			for _, taskDef := range tc.config.TaskDefinitions {
				taskDef.init()
				assert.Equal(t, []int{9404, 9406}, taskDef.metricsPortList)
			}
			targets := make(map[string]*Target)
			tc.task.addTargets(targets, tc.config)
			assert.Equal(t, tc.expectedTargets, targets)
		})
	}
}

func TestGetPrivateIp(t *testing.T) {
	testCases := []struct {
		testName string
		task     *ECSTask
		ip       string
	}{
		{
			"AWS VPC",
			buildWorkloadFargateAwsvpc(true, false),
			"10.0.0.129",
		},
		{
			"Bridge",
			buildWorkloadEC2BridgeDynamicPort(true, false),
			"10.4.0.205",
		},
		{
			"AWS VPC w/ no valid address",
			&ECSTask{
				Task: &ecs.Task{
					Attachments: []*ecs.Attachment{
						{
							Type: aws.String("ElasticNetworkInterface"),
							Details: []*ecs.KeyValuePair{
								{
									Name:  aws.String("networkInterfaceId"),
									Value: aws.String("eni-03de9d47faaa2e5ec"),
								},
							},
						},
						{
							Type: aws.String("invalid"),
						},
					},
				},
				TaskDefinition: &ecs.TaskDefinition{
					NetworkMode: aws.String(ecs.NetworkModeAwsvpc),
				},
			},
			"",
		},
		{
			"bridge w/ no EC2 Info",
			&ECSTask{
				TaskDefinition: &ecs.TaskDefinition{
					NetworkMode: aws.String(ecs.NetworkModeBridge),
				},
			},
			"",
		},
		{
			"no network mode",
			&ECSTask{
				TaskDefinition: &ecs.TaskDefinition{},
			},
			"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			ip := tc.task.getPrivateIP()
			assert.Equal(t, tc.ip, ip)
		})
	}
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
