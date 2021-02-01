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

package plugin

import (
	"fmt"

	"github.com/opentelemetry/opentelemetry-log-collection/errors"
)

const (
	stringType  = "string"
	boolType    = "bool"
	intType     = "int"
	stringsType = "strings"
	enumType    = "enum"
)

// Parameter is a basic description of a plugin's parameter.
type Parameter struct {
	Name        string `json:"name" yaml:"name"`
	Label       string `json:"label" yaml:"label"`
	Description string `json:"description" yaml:"description"`
	Required    bool   `json:"required" yaml:"required"`

	// "string", "int", "bool", "strings", or "enum"
	Type string `json:"type" yaml:"type"`

	// only useable if Type == "enum"
	ValidValues []string `json:"validValues" yaml:"valid_values"`

	// Must be valid according to Type & ValidValues
	Default    interface{}                       `json:"default" yaml:"default"`
	RelevantIf map[string]map[string]interface{} `json:"relevantIf" yaml:"relevant_if"`
}

func (p Parameter) validateValue(value interface{}) error {
	switch p.Type {
	case stringType:
		return p.validateStringValue(value)
	case intType:
		return p.validateIntValue(value)
	case boolType:
		return p.validateBoolValue(value)
	case stringsType:
		return p.validateStringsValue(value)
	case enumType:
		return p.validateEnumValue(value)
	default:
		return fmt.Errorf("invalid parameter type: %s", p.Type)
	}
}

func (p Parameter) validateStringValue(value interface{}) error {
	if _, ok := value.(string); !ok {
		return fmt.Errorf("parameter must be a string")
	}
	return nil
}

func (p Parameter) validateIntValue(value interface{}) error {
	if _, ok := value.(int); !ok {
		return fmt.Errorf("parameter must be an integer")
	}
	return nil
}

func (p Parameter) validateBoolValue(value interface{}) error {
	if _, ok := value.(bool); !ok {
		return fmt.Errorf("parameter must be a bool")
	}
	return nil
}

func (p Parameter) validateStringsValue(value interface{}) error {
	if _, ok := value.([]string); ok {
		return nil
	}

	array, ok := value.([]interface{})
	if !ok {
		return fmt.Errorf("parameter must be an array of strings")
	}

	for _, v := range array {
		if _, ok := v.(string); !ok {
			return fmt.Errorf("parameter contains a non string value")
		}
	}

	return nil
}

func (p Parameter) validateEnumValue(value interface{}) error {
	enum, ok := value.(string)
	if !ok {
		return fmt.Errorf("parameter must be a string")
	}

	for _, v := range p.ValidValues {
		if v == enum {
			return nil
		}
	}

	return fmt.Errorf("parameter must be one of the following values: %v", p.ValidValues)
}

func (p Parameter) validateDefinition() error {
	if err := p.validateType(); err != nil {
		return err
	}

	if err := p.validateValidValues(); err != nil {
		return err
	}

	if err := p.validateDefault(); err != nil {
		return err
	}

	// TODO validate relevant_if

	return nil
}

func (p Parameter) validateType() error {
	switch p.Type {
	case stringType, intType, boolType, stringsType, enumType: // ok
	default:
		return errors.NewError(
			"invalid type for parameter",
			"ensure that the type is one of 'string', 'int', 'bool', 'strings', or 'enum'",
		)
	}
	return nil
}

func (p Parameter) validateValidValues() error {
	switch p.Type {
	case stringType, intType, boolType, stringsType:
		if len(p.ValidValues) > 0 {
			return errors.NewError(
				fmt.Sprintf("valid_values is undefined for parameter of type '%s'", p.Type),
				"remove 'valid_values' field or change type to 'enum'",
			)
		}
	case enumType:
		if len(p.ValidValues) == 0 {
			return errors.NewError(
				"parameter of type 'enum' must have 'valid_values' specified",
				"specify an array that includes one or more valid values",
			)
		}
	}
	return nil
}

func (p Parameter) validateDefault() error {
	if p.Default == nil {
		return nil
	}

	// Validate that Default corresponds to Type
	switch p.Type {
	case stringType:
		return validateStringDefault(p)
	case intType:
		return validateIntDefault(p)
	case boolType:
		return validateBoolDefault(p)
	case stringsType:
		return validateStringArrayDefault(p)
	case enumType:
		return validateEnumDefault(p)
	default:
		return errors.NewError(
			"invalid type for parameter",
			"ensure that the type is one of 'string', 'int', 'bool', 'strings', or 'enum'",
		)
	}
}

func validateStringDefault(param Parameter) error {
	if _, ok := param.Default.(string); !ok {
		return errors.NewError(
			"default value for a parameter of type 'string' must be a string",
			"ensure that the default value is a string",
		)
	}
	return nil
}

func validateIntDefault(param Parameter) error {
	switch param.Default.(type) {
	case int, int32, int64:
		return nil
	default:
		return errors.NewError(
			"default value for a parameter of type 'int' must be an integer",
			"ensure that the default value is an integer",
		)
	}
}

func validateBoolDefault(param Parameter) error {
	if _, ok := param.Default.(bool); !ok {
		return errors.NewError(
			"default value for a parameter of type 'bool' must be a boolean",
			"ensure that the default value is a boolean",
		)
	}
	return nil
}

func validateStringArrayDefault(param Parameter) error {
	defaultList, ok := param.Default.([]interface{})
	if !ok {
		return errors.NewError(
			"default value for a parameter of type 'strings' must be an array of strings",
			"ensure that the default value is a string",
		)
	}
	for _, s := range defaultList {
		if _, ok := s.(string); !ok {
			return errors.NewError(
				"default value for a parameter of type 'strings' must be an array of strings",
				"ensure that the default value is an array of strings",
			)
		}
	}
	return nil
}

func validateEnumDefault(param Parameter) error {
	def, ok := param.Default.(string)
	if !ok {
		return errors.NewError(
			"invalid default for enumerated parameter",
			"ensure that the default value is a string",
		)
	}
	for _, val := range param.ValidValues {
		if val == def {
			return nil
		}
	}
	return errors.NewError(
		"invalid default value for enumerated parameter",
		"ensure default value is listed as a valid value",
	)
}
