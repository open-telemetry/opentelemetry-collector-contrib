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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/container"
)

func TestNewExtractionRule(t *testing.T) {
	var (
		config = &Config{
			ContainerLabelsToMetricLabels: map[string]string{
				"label_1": "label_1_tag",
			},
			EnvVarsToMetricLabels: map[string]string{
				"env_1": "env_1_tag",
			},
			Extract: ExtractConfig{
				Labels: []container.FieldExtractConfig{
					{
						TagName: "label_2_tag",
						Key:     "label_2",
					},
				},
				EnvVars: []container.FieldExtractConfig{
					{
						TagName: "env_2_tag",
						Key:     "env_2",
					},
				},
			},
		}

		expectedRules = container.ExtractionRules{
			EnvVars: []container.FieldExtractionRule{
				{
					Name: "env_1_tag",
					Key:  "env_1",
				},
				{
					Name: "env_2_tag",
					Key:  "env_2",
				},
			},
			Labels: []container.FieldExtractionRule{
				{
					Name: "label_1_tag",
					Key:  "label_1",
				},
				{
					Name: "label_2_tag",
					Key:  "label_2",
				},
			},
		}
	)

	actualRules, err := newExtractionRules(config)
	if assert.NoError(t, err) {
		assert.Equal(t, expectedRules, actualRules)
	}
}
