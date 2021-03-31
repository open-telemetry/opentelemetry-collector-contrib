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

package restructure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/errors"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("restructure", func() operator.Builder { return NewRestructureOperatorConfig("") })
}

// NewRestructureOperatorConfig creates a new restructure operator config with default values
func NewRestructureOperatorConfig(operatorID string) *RestructureOperatorConfig {
	return &RestructureOperatorConfig{
		TransformerConfig: helper.NewTransformerConfig(operatorID, "restructure"),
	}
}

// RestructureOperatorConfig is the configuration of a restructure operator
type RestructureOperatorConfig struct {
	helper.TransformerConfig `yaml:",inline"`

	Ops []Op `json:"ops" yaml:"ops"`
}

// Build will build a restructure operator from the supplied configuration
func (c RestructureOperatorConfig) Build(context operator.BuildContext) ([]operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(context)
	if err != nil {
		return nil, err
	}

	restructureOperator := &RestructureOperator{
		TransformerOperator: transformerOperator,
		ops:                 c.Ops,
	}

	return []operator.Operator{restructureOperator}, nil
}

// RestructureOperator is an operator that can restructure incoming entries using operations
type RestructureOperator struct {
	helper.TransformerOperator
	ops []Op
}

// Process will process an entry with a restructure transformation.
func (p *RestructureOperator) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Transform)
}

// Transform will apply the restructure operations to an entry
func (p *RestructureOperator) Transform(entry *entry.Entry) error {
	for _, op := range p.ops {
		err := op.Apply(entry)
		if err != nil {
			return err
		}
	}
	return nil
}

/*****************
  Op Definitions
*****************/

// Op is a designated operation on an entry
type Op struct {
	OpApplier
}

// OpApplier is an entity that applies an operation
type OpApplier interface {
	Apply(entry *entry.Entry) error
	Type() string
}

// UnmarshalJSON will unmarshal JSON into an operation
func (o *Op) UnmarshalJSON(raw []byte) error {
	var typeDecoder map[string]rawMessage
	err := json.Unmarshal(raw, &typeDecoder)
	if err != nil {
		return err
	}

	return o.unmarshalDecodedType(typeDecoder)
}

// UnmarshalYAML will unmarshal YAML into an operation
func (o *Op) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var typeDecoder map[string]rawMessage
	err := unmarshal(&typeDecoder)
	if err != nil {
		return err
	}

	return o.unmarshalDecodedType(typeDecoder)
}

type rawMessage struct {
	unmarshal func(interface{}) error
}

func (msg *rawMessage) UnmarshalYAML(unmarshal func(interface{}) error) error {
	msg.unmarshal = unmarshal
	return nil
}

func (msg *rawMessage) UnmarshalJSON(raw []byte) error {
	msg.unmarshal = func(dest interface{}) error {
		return json.Unmarshal(raw, dest)
	}
	return nil
}

func (msg *rawMessage) Unmarshal(v interface{}) error {
	return msg.unmarshal(v)
}

func (o *Op) unmarshalDecodedType(typeDecoder map[string]rawMessage) error {
	var rawMessage rawMessage
	var opType string
	for k, v := range typeDecoder {
		if opType != "" {
			return fmt.Errorf("only one Op type can be defined per operation")
		}
		opType = k
		rawMessage = v
	}

	if opType == "" {
		return fmt.Errorf("no Op type defined")
	}

	if rawMessage.unmarshal == nil {
		return fmt.Errorf("op fields cannot be empty")
	}

	opApplier, err := o.getOpApplier(opType, rawMessage)
	if err != nil {
		return err
	}

	o.OpApplier = opApplier
	return nil
}

func (o *Op) getOpApplier(opType string, rawMessage rawMessage) (OpApplier, error) {
	switch opType {
	case "move":
		var move OpMove
		err := rawMessage.Unmarshal(&move)
		return &move, err
	case "add":
		var add OpAdd
		err := rawMessage.Unmarshal(&add)
		return &add, err
	case "remove":
		var remove OpRemove
		err := rawMessage.Unmarshal(&remove)
		return &remove, err
	case "retain":
		var retain OpRetain
		err := rawMessage.Unmarshal(&retain)
		return &retain, err
	case "flatten":
		var flatten OpFlatten
		err := rawMessage.Unmarshal(&flatten)
		return &flatten, err
	default:
		return nil, fmt.Errorf("unknown op type '%s'", opType)
	}
}

// MarshalJSON will marshal an operation as JSON
func (o Op) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		o.Type(): o.OpApplier,
	})
}

// MarshalYAML will marshal an operation as YAML
func (o Op) MarshalYAML() (interface{}, error) {
	return map[string]interface{}{
		o.Type(): o.OpApplier,
	}, nil
}

/******
  Add
*******/

// OpAdd is an operation for adding fields to an entry
type OpAdd struct {
	Field     entry.Field `json:"field" yaml:"field"`
	Value     interface{} `json:"value,omitempty" yaml:"value,omitempty"`
	program   *vm.Program
	ValueExpr *string `json:"value_expr,omitempty" yaml:"value_expr,omitempty"`
}

// Apply will perform the add operation on an entry
func (op *OpAdd) Apply(e *entry.Entry) error {
	switch {
	case op.Value != nil:
		err := e.Set(op.Field, op.Value)
		if err != nil {
			return err
		}
	case op.program != nil:
		env := helper.GetExprEnv(e)
		defer helper.PutExprEnv(env)

		result, err := vm.Run(op.program, env)
		if err != nil {
			return fmt.Errorf("evaluate value_expr: %s", err)
		}
		err = e.Set(op.Field, result)
		if err != nil {
			return err
		}
	default:
		// Should never reach here if we went through the unmarshalling code
		return fmt.Errorf("neither value or value_expr are are set")
	}

	return nil
}

// Type will return the type of operation
func (op *OpAdd) Type() string {
	return "add"
}

type opAddRaw struct {
	Field     *entry.Field `json:"field"      yaml:"field"`
	Value     interface{}  `json:"value"      yaml:"value"`
	ValueExpr *string      `json:"value_expr" yaml:"value_expr"`
}

// UnmarshalJSON will unmarshal JSON into the add operation
func (op *OpAdd) UnmarshalJSON(raw []byte) error {
	var addRaw opAddRaw
	err := json.Unmarshal(raw, &addRaw)
	if err != nil {
		return fmt.Errorf("decode OpAdd: %s", err)
	}

	return op.unmarshalFromOpAddRaw(addRaw)
}

// UnmarshalYAML will unmarshal YAML into the add operation
func (op *OpAdd) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var addRaw opAddRaw
	err := unmarshal(&addRaw)
	if err != nil {
		return fmt.Errorf("decode OpAdd: %s", err)
	}

	return op.unmarshalFromOpAddRaw(addRaw)
}

func (op *OpAdd) unmarshalFromOpAddRaw(addRaw opAddRaw) error {
	if addRaw.Field == nil {
		return fmt.Errorf("decode OpAdd: missing required field 'field'")
	}

	switch {
	case addRaw.Value != nil && addRaw.ValueExpr != nil:
		return fmt.Errorf("decode OpAdd: only one of 'value' or 'value_expr' may be defined")
	case addRaw.Value == nil && addRaw.ValueExpr == nil:
		return fmt.Errorf("decode OpAdd: exactly one of 'value' or 'value_expr' must be defined")
	case addRaw.Value != nil:
		op.Field = *addRaw.Field
		op.Value = addRaw.Value
	case addRaw.ValueExpr != nil:
		compiled, err := expr.Compile(*addRaw.ValueExpr, expr.AllowUndefinedVariables())
		if err != nil {
			return fmt.Errorf("decode OpAdd: failed to compile expression '%s': %w", *addRaw.ValueExpr, err)
		}
		op.Field = *addRaw.Field
		op.program = compiled
		op.ValueExpr = addRaw.ValueExpr
	}

	return nil
}

/*********
  Remove
*********/

// OpRemove is operation for removing fields from an entry
type OpRemove struct {
	Field entry.Field
}

// Apply will perform the remove operation on an entry
func (op *OpRemove) Apply(e *entry.Entry) error {
	e.Delete(op.Field)
	return nil
}

// Type will return the type of operation
func (op *OpRemove) Type() string {
	return "remove"
}

// UnmarshalJSON will unmarshal JSON into a remove operation
func (op *OpRemove) UnmarshalJSON(raw []byte) error {
	return json.Unmarshal(raw, &op.Field)
}

// UnmarshalYAML will unmarshal YAML into a remove operation
func (op *OpRemove) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshal(&op.Field)
}

// MarshalJSON will marshal a remove operation into JSON
func (op OpRemove) MarshalJSON() ([]byte, error) {
	return json.Marshal(op.Field)
}

// MarshalYAML will marshal a remove operation into YAML
func (op OpRemove) MarshalYAML() (interface{}, error) {
	return op.Field.String(), nil
}

/*********
  Retain
*********/

// OpRetain is an operation for retaining fields
type OpRetain struct {
	Fields []entry.Field
}

// Apply will perform the retain operation on an entry
func (op *OpRetain) Apply(e *entry.Entry) error {
	newEntry := entry.New()
	newEntry.Timestamp = e.Timestamp
	for _, field := range op.Fields {
		val, ok := e.Get(field)
		if !ok {
			continue
		}
		err := newEntry.Set(field, val)
		if err != nil {
			return err
		}
	}
	*e = *newEntry
	return nil
}

// Type will return the type of operation
func (op *OpRetain) Type() string {
	return "retain"
}

// UnmarshalJSON will unmarshal JSON into a retain operation
func (op *OpRetain) UnmarshalJSON(raw []byte) error {
	return json.Unmarshal(raw, &op.Fields)
}

// UnmarshalYAML will unmarshal YAML into a retain operation
func (op *OpRetain) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshal(&op.Fields)
}

// MarshalJSON will marshal a retain operation into JSON
func (op OpRetain) MarshalJSON() ([]byte, error) {
	return json.Marshal(op.Fields)
}

// MarshalYAML will marshal a retain operation into YAML
func (op OpRetain) MarshalYAML() (interface{}, error) {
	return op.Fields, nil
}

/*******
  Move
*******/

// OpMove is an operation for moving entry fields
type OpMove struct {
	From entry.Field `json:"from" yaml:"from,flow"`
	To   entry.Field `json:"to" yaml:"to,flow"`
}

// Apply will perform the move operation on an entry
func (op *OpMove) Apply(e *entry.Entry) error {
	val, ok := e.Delete(op.From)
	if !ok {
		return fmt.Errorf("apply move: field %s does not exist on record", op.From)
	}

	return e.Set(op.To, val)
}

// Type will return the type of operation
func (op *OpMove) Type() string {
	return "move"
}

/**********
  Flatten
**********/

// OpFlatten is an operation for flattening fields
type OpFlatten struct {
	Field entry.RecordField
}

// Apply will perform the flatten operation on an entry
func (op *OpFlatten) Apply(e *entry.Entry) error {
	parent := op.Field.Parent()
	val, ok := e.Delete(op.Field)
	if !ok {
		// The field doesn't exist, so ignore it
		return fmt.Errorf("apply flatten: field %s does not exist on record", op.Field)
	}

	valMap, ok := val.(map[string]interface{})
	if !ok {
		// The field we were asked to flatten was not a map, so put it back
		err := e.Set(op.Field, val)
		if err != nil {
			return errors.Wrap(err, "reset non-map field")
		}
		return fmt.Errorf("apply flatten: field %s is not a map", op.Field)
	}

	for k, v := range valMap {
		err := e.Set(parent.Child(k), v)
		if err != nil {
			return err
		}
	}
	return nil
}

// Type will return the type of operation
func (op *OpFlatten) Type() string {
	return "flatten"
}

// UnmarshalJSON will unmarshal JSON into a flatten operation
func (op *OpFlatten) UnmarshalJSON(raw []byte) error {
	return json.Unmarshal(raw, &op.Field)
}

// UnmarshalYAML will unmarshal YAML into a flatten operation
func (op *OpFlatten) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshal(&op.Field)
}

// MarshalJSON will marshal a flatten operation into JSON
func (op OpFlatten) MarshalJSON() ([]byte, error) {
	return json.Marshal(op.Field)
}

// MarshalYAML will marshal a flatten operation into YAML
func (op OpFlatten) MarshalYAML() (interface{}, error) {
	return op.Field.String(), nil
}
