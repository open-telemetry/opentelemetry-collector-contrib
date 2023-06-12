// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"

import (
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"go.uber.org/zap"
)

// DockerLabelConfig matches all tasks based on their docker label.
//
// NOTE: it's possible to make DockerLabelConfig part of CommonExporterConfig
// and use it both ServiceConfig and TaskDefinitionConfig.
// However, based on existing users, few people mix different types of filters.
// If that usecase arises in the future, we can rewrite the top level docker lable filter
// using a task definition filter with arn_pattern:*.
type DockerLabelConfig struct {
	CommonExporterConfig `mapstructure:",squash" yaml:",inline"`

	// PortLabel is mandatory, empty string means docker label based match is skipped.
	PortLabel        string `mapstructure:"port_label" yaml:"port_label"`
	JobNameLabel     string `mapstructure:"job_name_label" yaml:"job_name_label"`
	MetricsPathLabel string `mapstructure:"metrics_path_label" yaml:"metrics_path_label"`
}

func (d *DockerLabelConfig) validate() error {
	_, err := d.newMatcher(matcherOptions{})
	return err
}

func (d *DockerLabelConfig) newMatcher(options matcherOptions) (targetMatcher, error) {
	// It's possible to support it in the future, but for now just fail at config,
	// so user don't need to wonder which port is used in the exported target.
	if len(d.MetricsPorts) != 0 {
		return nil, fmt.Errorf("metrics_ports is not supported in docker_labels, got %v", d.MetricsPorts)
	}
	if d.PortLabel == "" {
		return nil, fmt.Errorf("port_label is empty")
	}
	expSetting, err := d.newExportSetting()
	if err != nil {
		return nil, err
	}
	return &dockerLabelMatcher{
		logger:        options.Logger,
		cfg:           *d,
		exportSetting: expSetting,
	}, nil
}

func dockerLabelConfigToMatchers(cfgs []DockerLabelConfig) []matcherConfig {
	var matchers []matcherConfig
	for _, cfg := range cfgs {
		// NOTE: &cfg points to the temp var, whose value would end up be the last one in the slice.
		copied := cfg
		matchers = append(matchers, &copied)
	}
	return matchers
}

// dockerLabelMatcher implements targetMatcher interface.
// It checks PortLabel from config and only matches if the label value is a valid number.
type dockerLabelMatcher struct {
	logger        *zap.Logger
	cfg           DockerLabelConfig
	exportSetting *commonExportSetting
}

func (d *dockerLabelMatcher) matcherType() matcherType {
	return matcherTypeDockerLabel
}

// MatchTargets first checks the port label to find the expected port value.
// Then it checks if that port is specified in container definition.
// It only returns match target when both conditions are met.
func (d *dockerLabelMatcher) matchTargets(_ *taskAnnotated, c *ecs.ContainerDefinition) ([]matchedTarget, error) {
	portLabel := d.cfg.PortLabel

	// Only check port label
	ps, ok := c.DockerLabels[portLabel]
	if !ok {
		return nil, errNotMatched
	}

	// Convert port
	s := aws.StringValue(ps)
	port, err := strconv.Atoi(s)
	if err != nil {
		return nil, fmt.Errorf("invalid port_label value, container=%s labelKey=%s labelValue=%s: %w",
			aws.StringValue(c.Name), d.cfg.PortLabel, s, err)
	}

	// Checks if the task does have the container port
	portExists := false
	for _, portMapping := range c.PortMappings {
		if aws.Int64Value(portMapping.ContainerPort) == int64(port) {
			portExists = true
			break
		}
	}
	if !portExists {
		return nil, errNotMatched
	}

	// Export only one target based on docker port label.
	target := matchedTarget{
		Port: port,
	}
	if v, ok := c.DockerLabels[d.cfg.MetricsPathLabel]; ok {
		target.MetricsPath = aws.StringValue(v)
	}
	if v, ok := c.DockerLabels[d.cfg.JobNameLabel]; ok {
		target.Job = aws.StringValue(v)
	}
	// NOTE: we only override job name but keep port and metrics from docker label instead of using common export config.
	if d.cfg.JobName != "" {
		target.Job = d.cfg.JobName
	}
	return []matchedTarget{target}, nil
}
