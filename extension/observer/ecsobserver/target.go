// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// target.go defines labels and structs in exported target.

const (
	labelPrefix = "__meta_ecs_"
)

// prometheusECSTarget contains address and labels extracted from a running ECS task
// and its underlying EC2 instance (if available).
//
// For serialization
// - TargetToLabels and LabelsToTarget converts the struct between map[string]string.
// - TargetsToFileSDYAML and ToTargetYAML converts it between prometheus file discovery format in YAML.
type prometheusECSTarget struct {
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

// ToLabels converts fields in the target to map.
// It also sanitize label name because the requirements on AWS tags and Prometheus are different.
func (t *prometheusECSTarget) ToLabels() map[string]string {
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
	trimEmptyValueByKeyPrefix(labels, labelPrefix+"ec2_")
	addTagsToLabels(t.TaskTags, labelPrefixTaskTags, labels)
	addTagsToLabels(t.ContainerLabels, labelPrefixContainerLabels, labels)
	addTagsToLabels(t.EC2Tags, labelPrefixEC2Tags, labels)
	return labels
}

// addTagsToLabels merge tags (from ecs, ec2 etc.) into existing labels.
// tag key are prefixed with labelNamePrefix and sanitize with sanitizeLabelName.
func addTagsToLabels(tags map[string]string, labelNamePrefix string, labels map[string]string) {
	for k, v := range tags {
		labels[labelNamePrefix+"_"+sanitizeLabelName(k)] = v
	}
}

func trimEmptyValueByKeyPrefix(m map[string]string, prefix string) {
	for k, v := range m {
		if v == "" && strings.HasPrefix(k, prefix) {
			delete(m, k)
		}
	}
}

var invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// Copied from https://github.com/prometheus/prometheus/blob/8d2a8f493905e46fe6181e8c1b79ccdfcbdb57fc/util/strutil/strconv.go#L40-L44
func sanitizeLabelName(s string) string {
	return invalidLabelCharRE.ReplaceAllString(s, "_")
}

type fileSDTarget struct {
	Targets []string          `yaml:"targets" json:"targets"`
	Labels  map[string]string `yaml:"labels" json:"labels"`
}

func targetsToFileSDTargets(targets []prometheusECSTarget, jobLabelName string) ([]fileSDTarget, error) {
	converted := make([]fileSDTarget, len(targets))
	omitEmpty := []string{labelJob, labelServiceName}
	for i, t := range targets {
		labels := t.ToLabels()
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
		// Rename job label as a workaround for https://github.com/open-telemetry/opentelemetry-collector/issues/575#issuecomment-814558584
		// In order to keep similar behavior as cloudwatch agent's discovery implementation,
		// we support getting job name from docker label. However, prometheus receiver is using job and __name__
		// labels to get metric type, and it believes the job specified in prom config is always the same as
		// the job label attached to metrics. Prometheus itself allows discovery to provide job names.
		//
		// We can't relabel it using prometheus's relabel config as it would cause the same problem on receiver.
		// We 'relabel' it to job outside prometheus receiver using other processors in collector's pipeline.
		job := labels[labelJob]
		if job != "" && jobLabelName != labelJob {
			delete(labels, labelJob)
			labels[jobLabelName] = job
		}
		pt := fileSDTarget{
			Targets: []string{address},
			Labels:  labels,
		}
		converted[i] = pt
	}
	return converted, nil
}

func targetsToFileSDYAML(targets []prometheusECSTarget, jobLabelName string) ([]byte, error) {
	converted, err := targetsToFileSDTargets(targets, jobLabelName)
	if err != nil {
		return nil, err
	}
	b, err := yaml.Marshal(converted)
	if err != nil {
		return nil, fmt.Errorf("encode targets as YAML failed: %w", err)
	}
	return b, nil
}
