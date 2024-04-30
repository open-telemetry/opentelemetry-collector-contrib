// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pipeline // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

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
