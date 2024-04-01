// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pipeline // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

type Option func(*pipelineOpts)

func WithDefaultOutput(op operator.Operator) Option {
	return func(p *pipelineOpts) {
		p.defaultOutput = op
	}
}

type pipelineOpts struct {
	defaultOutput operator.Operator
}

func New(set component.TelemetrySettings, opCfgs []operator.Identity, opts ...Option) (*DirectedPipeline, error) {
	if set.Logger == nil {
		return nil, errors.NewError("logger must be provided", "")
	}
	if len(opCfgs) == 0 {
		return nil, errors.NewError("operators must be specified", "")
	}

	pOpts := &pipelineOpts{}
	for _, opt := range opts {
		opt(pOpts)
	}

	dedeplucateComponentIDs(opCfgs)

	ops := make([]operator.Operator, 0, len(opCfgs))
	for _, opCfg := range opCfgs {
		f, ok := operator.GlobalFactoryRegistry.LookupFactory(opCfg.ComponentID().Type())
		if !ok {
			return nil, fmt.Errorf("unknown factory type %q", opCfg.ComponentID().Type())
		}
		op, err := f.CreateOperator(set, opCfg)
		if err != nil {
			return nil, fmt.Errorf("create operator %q: %w", opCfg, err)
		}
		ops = append(ops, op)
	}

	for i, op := range ops {
		// Any operator that already has an output will not be changed
		if len(op.GetOutputIDs()) > 0 {
			continue
		}

		// Any operator (except the last) will just output to the next
		if i+1 < len(ops) {
			op.SetOutputIDs([]string{ops[i+1].ID()})
			continue
		}

		// The last operator may output to the default output
		if op.CanOutput() && pOpts.defaultOutput != nil {
			ops = append(ops, pOpts.defaultOutput)
			op.SetOutputIDs([]string{ops[i+1].ID()})
		}
	}

	return NewDirectedPipeline(ops)
}

func dedeplucateComponentIDs(ids []operator.Identity) {
	typeMap := make(map[component.Type]int)
	for _, id := range ids {
		if id.ComponentID().Name() != "" {
			continue
		}

		t := id.ComponentID().Type()
		if typeMap[t] == 0 {
			typeMap[t]++
			continue
		}
		indexedName := fmt.Sprintf("%s%d", t, typeMap[t])

		for j := 0; j < len(ids); j++ {
			if ids[j].ComponentID().Type() == t && ids[j].ComponentID().Name() == indexedName {
				j = 0
				typeMap[t]++
				indexedName = fmt.Sprintf("%s%d", t, typeMap[t])
			}
		}

		typeMap[t]++
		id.SetName(indexedName)
	}
}

// Config is the configuration of a pipeline.
type Config struct {
	DefaultOutput operator.Operator
	Operators     []operator.Config
}

// Build will build a pipeline from the config.
func (c Config) Build(logger *zap.SugaredLogger) (*DirectedPipeline, error) {
	if logger == nil {
		return nil, errors.NewError("logger must be provided", "")
	}
	if c.Operators == nil {
		return nil, errors.NewError("operators must be specified", "")
	}

	if len(c.Operators) == 0 {
		return nil, errors.NewError("empty pipeline not allowed", "")
	}

	dedeplucateIDs(c.Operators)

	ops := make([]operator.Operator, 0, len(c.Operators))
	for _, opCfg := range c.Operators {
		op, err := opCfg.Build(logger)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}

	for i, op := range ops {
		// Any operator that already has an output will not be changed
		if len(op.GetOutputIDs()) > 0 {
			continue
		}

		// Any operator (except the last) will just output to the next
		if i+1 < len(ops) {
			op.SetOutputIDs([]string{ops[i+1].ID()})
			continue
		}

		// The last operator may output to the default output
		if op.CanOutput() && c.DefaultOutput != nil {
			ops = append(ops, c.DefaultOutput)
			op.SetOutputIDs([]string{ops[i+1].ID()})
		}
	}

	return NewDirectedPipeline(ops)
}

func dedeplucateIDs(ops []operator.Config) {
	typeMap := make(map[string]int)
	for _, op := range ops {
		if op.Type() != op.ID() {
			continue
		}

		if typeMap[op.Type()] == 0 {
			typeMap[op.Type()]++
			continue
		}
		newID := fmt.Sprintf("%s%d", op.Type(), typeMap[op.Type()])

		for j := 0; j < len(ops); j++ {
			if newID == ops[j].ID() {
				j = 0
				typeMap[op.Type()]++
				newID = fmt.Sprintf("%s%d", op.Type(), typeMap[op.Type()])
			}
		}

		typeMap[op.Type()]++
		op.SetID(newID)
	}
}
