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

package ecsobserver

import (
	"fmt"
	"regexp"
	"strconv"

	"gopkg.in/yaml.v2"
)

// target.go defines labels and structs in exported target.

const (
	labelPrefix = "__meta_ecs_"
)

// PrometheusECSTarget contains address and labels extracted from a running ECS task
// and its underlying EC2 instance (if available).
//
// For serialization
// - TargetToLabels and LabelsToTarget converts the struct between map[string]string.
// - TargetsToFileSDYAML and ToTargetYAML converts it between prometheus file discovery format in YAML.
type PrometheusECSTarget struct {
	Source                 string            `label:"source"`
	Address                string            `label:"__address__"`
	MetricsPath            string            `label:"__metrics_path__"`
	Job                    string            `label:"job"`
	ClusterName            string            `label:"cluster_name"`
	ServiceName            string            `label:"service_name"`
	TaskDefinitionFamily   string            `label:"task_definition_family"`
	TaskDefinitionRevision int               `label:"task_definition_revision"`
	TaskStartedBy          string            `label:"task_started_by"`
	TaskLaunchType         string            `label:"task_launch_type"`
	TaskGroup              string            `label:"task_group"`
	TaskTags               map[string]string `label:"task_tags"`
	ContainerName          string            `label:"container_name"`
	ContainerLabels        map[string]string `label:"container_labels"`
	HealthStatus           string            `label:"health_status"`
	EC2InstanceID          string            `label:"ec2_instance_id"`
	EC2InstanceType        string            `label:"ec2_instance_type"`
	EC2Tags                map[string]string `label:"ec2_tags"`
	EC2VpcID               string            `label:"ec2_vpc_id"`
	EC2SubnetID            string            `label:"ec2_subnet_id"`
	EC2PrivateIP           string            `label:"ec2_private_ip"`
	EC2PublicIP            string            `label:"ec2_public_ip"`
}

const (
	labelSource                 = labelPrefix + "source"
	labelAddress                = "__address__"
	labelMetricsPath            = "__metrics_path__"
	labelJob                    = "job"
	labelClusterName            = labelPrefix + "cluster_name"
	labelServiceName            = labelPrefix + "service_name"
	labelTaskDefinitionFamily   = labelPrefix + "task_definition_family"
	labelTaskDefinitionRevision = labelPrefix + "task_definition_revision"
	labelTaskStartedBy          = labelPrefix + "task_started_by"
	labelTaskLaunchType         = labelPrefix + "task_launch_type"
	labelTaskGroup              = labelPrefix + "task_group"
	labelPrefixTaskTags         = labelPrefix + "task_tags"
	labelContainerName          = labelPrefix + "container_name"
	labelPrefixContainerLabels  = labelPrefix + "container_labels"
	labelHealthStatus           = labelPrefix + "health_status"
	labelEC2InstanceID          = labelPrefix + "ec2_instance_id"
	labelEC2InstanceType        = labelPrefix + "ec2_instance_type"
	labelPrefixEC2Tags          = labelPrefix + "ec2_tags"
	labelEC2VpcID               = labelPrefix + "ec2_vpc_id"
	labelEC2SubnetID            = labelPrefix + "ec2_subnet_id"
	labelEC2PrivateIP           = labelPrefix + "ec2_private_ip"
	labelEC2PublicIP            = labelPrefix + "ec2_public_ip"
)

func TargetToLabels(t PrometheusECSTarget) map[string]string {
	labels := map[string]string{
		labelSource:                 t.Source,
		labelAddress:                t.Address,
		labelMetricsPath:            t.MetricsPath,
		labelJob:                    t.Job,
		labelClusterName:            t.ClusterName,
		labelServiceName:            t.ServiceName,
		labelTaskDefinitionFamily:   t.TaskDefinitionFamily,
		labelTaskDefinitionRevision: strconv.Itoa(t.TaskDefinitionRevision),
		labelTaskStartedBy:          t.TaskStartedBy,
		labelTaskLaunchType:         t.TaskLaunchType,
		labelTaskGroup:              t.TaskGroup,
		labelContainerName:          t.ContainerName,
		labelHealthStatus:           t.HealthStatus,
		labelEC2InstanceID:          t.EC2InstanceID,
		labelEC2InstanceType:        t.EC2InstanceType,
		labelEC2VpcID:               t.EC2VpcID,
		labelEC2SubnetID:            t.EC2SubnetID,
		labelEC2PrivateIP:           t.EC2PrivateIP,
		labelEC2PublicIP:            t.EC2PublicIP,
	}
	addTagsToLabels(t.TaskTags, labelPrefixTaskTags, labels)
	addTagsToLabels(t.ContainerLabels, labelPrefixContainerLabels, labels)
	addTagsToLabels(t.EC2Tags, labelPrefixEC2Tags, labels)
	return labels
}

type FileSDTarget struct {
	Targets []string          `yaml:"targets" json:"targets"`
	Labels  map[string]string `yaml:"labels" json:"labels"`
}

func TargetsToFileSDTargets(targets []PrometheusECSTarget, jobLabelName string) ([]FileSDTarget, error) {
	var converted []FileSDTarget
	omitEmpty := []string{labelJob, labelServiceName}
	for _, t := range targets {
		labels := TargetToLabels(t)
		address, ok := labels[labelAddress]
		if !ok {
			return nil, fmt.Errorf("address label not found for %v", labels)
		}
		delete(labels, labelAddress)
		// Remove some labels if their value is empty
		for _, k := range omitEmpty {
			if v, ok := labels[k]; ok && v == "" {
				delete(labels, k)
			}
		}
		// Rename job label as a workaround for https://github.com/open-telemetry/opentelemetry-collector/issues/575
		job := labels[labelJob]
		if job != "" && jobLabelName != labelJob {
			delete(labels, labelJob)
			labels[jobLabelName] = job
		}
		pt := FileSDTarget{
			Targets: []string{address},
			Labels:  labels,
		}
		converted = append(converted, pt)
	}
	return converted, nil
}

func TargetsToFileSDYAML(targets []PrometheusECSTarget, jobLabelName string) ([]byte, error) {
	converted, err := TargetsToFileSDTargets(targets, jobLabelName)
	if err != nil {
		return nil, err
	}
	b, err := yaml.Marshal(converted)
	if err != nil {
		return nil, fmt.Errorf("encode targets as YAML failed: %w", err)
	}
	return b, nil
}

// addTagsToLabels merge tags (from ecs, ec2 etc.) into existing labels.
// tag key are prefixed with labelNamePrefix and sanitize with sanitizeLabelName.
func addTagsToLabels(tags map[string]string, labelNamePrefix string, labels map[string]string) {
	for k, v := range tags {
		labels[labelNamePrefix+"_"+sanitizeLabelName(k)] = v
	}
}

var (
	invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

// Copied from https://github.com/prometheus/prometheus/blob/8d2a8f493905e46fe6181e8c1b79ccdfcbdb57fc/util/strutil/strconv.go#L40-L44
func sanitizeLabelName(s string) string {
	return invalidLabelCharRE.ReplaceAllString(s, "_")
}
