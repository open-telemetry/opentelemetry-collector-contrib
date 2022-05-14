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

package pipeline // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"

import (
	"hash/fnv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// OperatorNode is a basic node that represents an operator in a pipeline.
type OperatorNode struct {
	operator  operator.Operator
	outputIDs map[string]int64
	id        int64
}

// Operator returns the operator of the node.
func (b OperatorNode) Operator() operator.Operator {
	return b.operator
}

// ID returns the node id.
func (b OperatorNode) ID() int64 {
	return b.id
}

// DOTID returns the id used to represent this node in a dot graph.
func (b OperatorNode) DOTID() string {
	return b.operator.ID()
}

// OutputIDs returns a map of output operator ids to node ids.
func (b OperatorNode) OutputIDs() map[string]int64 {
	return b.outputIDs
}

// createOperatorNode will create an operator node.
func createOperatorNode(operator operator.Operator) OperatorNode {
	id := createNodeID(operator.ID())
	outputIDs := make(map[string]int64)
	if operator.CanOutput() {
		for _, output := range operator.Outputs() {
			outputIDs[output.ID()] = createNodeID(output.ID())
		}
	}
	return OperatorNode{
		operator:  operator,
		outputIDs: outputIDs,
		id:        id,
	}
}

// createNodeID generates a node id from an operator id.
func createNodeID(operatorID string) int64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(operatorID))
	return int64(hash.Sum64())
}
