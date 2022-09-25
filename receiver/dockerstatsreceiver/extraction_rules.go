// Copyright 2020 OpenTelemetry Authors
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

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/container"
)

func newExtractionRules(config *Config) (rules container.ExtractionRules, err error) {
	for containerLabel, attrName := range config.ContainerLabelsToMetricLabels {
		rules.Labels = append(rules.Labels, container.FieldExtractionRule{
			Name: attrName,
			Key:  containerLabel,
		})
	}
	for containerEnvVar, attrName := range config.EnvVarsToMetricLabels {
		rules.EnvVars = append(rules.EnvVars, container.FieldExtractionRule{
			Name: attrName,
			Key:  containerEnvVar,
		})
	}

	envVarsExtractionRules, err := container.ExtractFieldRules("env_vars", config.Extract.EnvVars)
	if err != nil {
		return container.ExtractionRules{}, fmt.Errorf("failed to extract env vars field rules %w", err)
	}
	rules.EnvVars = append(rules.EnvVars, envVarsExtractionRules...)

	labelsExtractionRules, err := container.ExtractFieldRules("labels", config.Extract.Labels)
	if err != nil {
		return container.ExtractionRules{}, fmt.Errorf("failed to extract labels field rules %w", err)
	}
	rules.Labels = append(rules.Labels, labelsExtractionRules...)

	return rules, nil
}
