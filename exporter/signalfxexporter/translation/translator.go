// Copyright 2019, OpenTelemetry Authors
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

package translation

import (
	"fmt"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
)

// Action is the enum to capture actions to perform on metrics.
type Action string

const (
	// ActionRenameDimensionKeys renames dimension keys using Rule.Mapping.
	ActionRenameDimensionKeys Action = "rename_dimension_keys"
)

type Rule struct {
	// Action specifies the translation action to be applied on metrics.
	// This is a required field.
	Action Action `mapstructure:"action"`

	// Mapping specifies key/value mapping that is used by rename_dimension_keys action.
	Mapping map[string]string `mapstructure:"mapping"`
}

type MetricTranslator struct {
	rules []Rule

	// Additional map to be used only for dimension renaming in metadata
	dimensionsMap map[string]string
}

func NewMetricTranslator(rules []Rule) (*MetricTranslator, error) {
	err := validateTranslationRules(rules)
	if err != nil {
		return nil, err
	}

	return &MetricTranslator{
		rules:         rules,
		dimensionsMap: createDimensionsMap(rules),
	}, nil
}

func validateTranslationRules(rules []Rule) error {
	var renameDimentionKeysFound bool
	for _, tr := range rules {
		switch tr.Action {
		case ActionRenameDimensionKeys:
			if tr.Mapping == nil {
				return fmt.Errorf("field \"mapping\" is required for %q translation rule", tr.Action)
			}
			if renameDimentionKeysFound {
				return fmt.Errorf("only one %q translation rule can be specified", tr.Action)
			}
			renameDimentionKeysFound = true
		default:
			return fmt.Errorf("unknown \"action\" value: %q", tr.Action)
		}
	}
	return nil
}

// createDimensionsMap creates an additional map for dimensions
// from ActionRenameDimensionKeys actions in rules.
func createDimensionsMap(rules []Rule) map[string]string {
	for _, tr := range rules {
		if tr.Action == ActionRenameDimensionKeys {
			return tr.Mapping
		}
	}

	return nil
}

func (mp *MetricTranslator) TranslateDataPoints(sfxDataPoints []*sfxpb.DataPoint) {
	for _, tr := range mp.rules {
		for _, dp := range sfxDataPoints {
			switch tr.Action {
			case ActionRenameDimensionKeys:
				for _, d := range dp.Dimensions {
					if newKey, ok := tr.Mapping[d.Key]; ok {
						d.Key = newKey
					}
				}
			}
		}
	}
}

func (mp *MetricTranslator) TranslateDimension(orig string) string {
	if translated, ok := mp.dimensionsMap[orig]; ok {
		return translated
	}
	return orig
}
