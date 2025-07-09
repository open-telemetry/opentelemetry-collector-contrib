// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
