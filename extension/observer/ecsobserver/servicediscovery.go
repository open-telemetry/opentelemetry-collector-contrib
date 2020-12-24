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
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

const (
	AwsSdkLevelRetryCount = 3

	//https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
	defaultPrometheusMetricsPath = "/metrics"
)

// PrometheusTarget represents a discovered Prometheus target.
type PrometheusTarget struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels"`
}

// Target represents a discovered ECS endpoint.
type Target struct {
	Address     string
	MetricsPath string
	Labels      map[string]string
}

// toPrometheusTarget converts target into seriablizable Prometheus target.
func (t *Target) toPrometheusTarget() *PrometheusTarget {
	metricsPath := t.MetricsPath
	if metricsPath == "" {
		metricsPath = defaultPrometheusMetricsPath
	}

	return &PrometheusTarget{
		Targets: []string{t.Address + metricsPath},
		Labels:  t.Labels,
	}
}

// toEndpoint converts target into seriablizable Prometheus target.
func (t *Target) toEndpoint() *observer.Endpoint {
	return &observer.Endpoint{
		ID:     observer.EndpointID(t.Address + t.MetricsPath),
		Target: t.Address,
		Details: observer.Task{
			MetricsPath: t.MetricsPath,
			Labels:      t.Labels,
		},
	}
}

type serviceDiscovery struct {
	config *Config

	svcEcs *ecs.ECS
	svcEc2 *ec2.EC2

	processors []Processor
}

func (sd *serviceDiscovery) init() {
	region := getAWSRegion(sd.config)
	awsConfig := aws.NewConfig().WithRegion(region).WithMaxRetries(AwsSdkLevelRetryCount)
	session := session.New(awsConfig)
	sd.svcEcs = ecs.New(session, awsConfig)
	sd.svcEc2 = ec2.New(session, awsConfig)
	sd.initProcessors()
}

func (sd *serviceDiscovery) initProcessors() {
	sd.processors = []Processor{
		NewTaskRetrievalProcessor(sd.svcEcs, sd.config.logger),
		NewTaskDefinitionProcessor(sd.svcEcs),
		NewTaskFilterProcessor(sd.config.TaskDefinitions, sd.config.DockerLabel),
		NewMetadataProcessor(sd.svcEcs, sd.svcEc2, sd.config.logger),
	}
}

func (sd *serviceDiscovery) discoverTargets() map[string]*Target {
	var taskList []*ECSTask
	var err error
	for _, p := range sd.processors {
		taskList, err = p.Process(sd.config.ClusterName, taskList)
		// Ignore partial result to avoid overwriting existing targets
		if err != nil {
			sd.config.logger.Error(
				"ECS Service Discovery failed to discover targets",
				zap.String("processor name", p.ProcessorName()),
				zap.String("error", err.Error()),
			)
			return nil
		}
	}

	// Dedup Key for Targets: target + metricsPath
	// e.g. 10.0.0.28:9404/metrics
	//      10.0.0.28:9404/stats/metrics
	targets := make(map[string]*Target)
	for _, t := range taskList {
		t.addTargets(targets, sd.config)
	}

	return targets
}

// getAWSRegion retrieves the AWS region from the provided config, env var, or EC2 metadata.
func getAWSRegion(cfg *Config) (awsRegion string) {
	awsRegion = cfg.ClusterRegion

	if awsRegion == "" {
		if regionEnv := os.Getenv("AWS_REGION"); regionEnv != "" {
			awsRegion = regionEnv
			cfg.logger.Debug("Cluster region not defined. Fetched region from environment variables", zap.String("region", awsRegion))
		} else {
			if s, err := session.NewSession(); err != nil {
				cfg.logger.Error("Unable to create default session", zap.Error(err))
			} else {
				awsRegion, err = ec2metadata.New(s).Region()
				if err != nil {
					cfg.logger.Error("Unable to retrieve the region from the EC2 instance", zap.Error(err))
				} else {
					cfg.logger.Debug("Fetch region from EC2 metadata", zap.String("region", awsRegion))
				}
			}
		}
	}

	if awsRegion == "" {
		cfg.logger.Error("Cannot fetch region variable from config file, environment variables, or ec2 metadata.")
	}

	return
}
