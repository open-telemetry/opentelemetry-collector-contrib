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

package pipeline

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/operator"
)

// Config is the configuration of a pipeline.
type Config []operator.Config

// Build will build a pipeline from the config.
func (c Config) Build(logger *zap.SugaredLogger, defaultOut operator.Operator) (*DirectedPipeline, error) {
	c.dedeplucateIDs()

	ops := make([]operator.Operator, 0, len(c))
	for _, builder := range c {
		op, err := builder.Build(logger)
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
		if op.CanOutput() && defaultOut != nil {
			ops = append(ops, defaultOut)
			op.SetOutputIDs([]string{ops[i+1].ID()})
		}
	}

	return NewDirectedPipeline(ops)
}

func (c Config) dedeplucateIDs() {
	typeMap := make(map[string]int)
	for _, op := range c {
		if op.Type() != op.ID() {
			continue
		}

		if typeMap[op.Type()] == 0 {
			typeMap[op.Type()]++
			continue
		}
		newID := fmt.Sprintf("%s%d", op.Type(), typeMap[op.Type()])

		for j := 0; j < len(c); j++ {
			if newID == c[j].ID() {
				j = 0
				typeMap[op.Type()]++
				newID = fmt.Sprintf("%s%d", op.Type(), typeMap[op.Type()])
			}
		}

		typeMap[op.Type()]++
		op.SetID(newID)
	}
}
