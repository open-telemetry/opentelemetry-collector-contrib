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
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

type PrometheusTarget struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels"`
}

// TargetProcessor converts ECS tasks to Prometheus targets.
type TargetProcessor struct {
	tmpResultFilePath string
	config            *Config
}

func (p *TargetProcessor) ProcessorName() string {
	return "TargetProcessor"
}

// Process generates the scrape targets from discovered tasks.
func (p *TargetProcessor) Process(clusterName string, taskList []*ECSTask) ([]*ECSTask, error) {
	// Dedup Key for Targets: target + metricsPath
	// e.g. 10.0.0.28:9404/metrics
	//      10.0.0.28:9404/stats/metrics
	targets := make(map[string]*PrometheusTarget)
	for _, t := range taskList {
		t.addPrometheusTargets(targets, p.config)
	}

	targetsArr := make([]*PrometheusTarget, len(targets))
	idx := 0
	for _, target := range targets {
		targetsArr[idx] = target
		idx++
	}

	m, err := yaml.Marshal(targetsArr)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal Prometheus targets. Error: %s", err.Error())
	}

	err = ioutil.WriteFile(p.tmpResultFilePath, m, 0644)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal Prometheus targets into file: %s. Error: %s", p.tmpResultFilePath, err.Error())
	}

	err = os.Rename(p.tmpResultFilePath, p.config.ResultFile)
	if err != nil {
		os.Remove(p.tmpResultFilePath)
		return nil, fmt.Errorf("Failed to rename tmp result file %s to %s. Error: %s", p.tmpResultFilePath, p.config.ResultFile, err.Error())
	}

	return nil, nil
}

func NewTargetProcessor(config *Config) *TargetProcessor {
	return &TargetProcessor{
		tmpResultFilePath: config.ResultFile + "_temp",
		config:            config,
	}
}
