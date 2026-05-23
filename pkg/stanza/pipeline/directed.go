// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pipeline // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/multierr"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/stanzaerrors"
)

var _ Pipeline = (*DirectedPipeline)(nil)

var (
	errAlreadyStarted = errors.New("pipeline already started")
	errAlreadyStopped = errors.New("pipeline already stopped")
)

// DirectedPipeline is a pipeline backed by a directed graph
type DirectedPipeline struct {
	Graph     *simple.DirectedGraph
	startOnce sync.Once
	stopOnce  sync.Once
}

// Start will start the operators in a pipeline in reverse topological order
func (p *DirectedPipeline) Start(persister operator.Persister) error {
	err := errAlreadyStarted
	p.startOnce.Do(func() {
		err = p.start(persister)
	})
	return err
}

// Stop will stop the operators in a pipeline in topological order
func (p *DirectedPipeline) Stop() error {
	err := errAlreadyStopped
	p.stopOnce.Do(func() {
		err = p.stop()
	})
	return err
}

func (p *DirectedPipeline) start(persister operator.Persister) error {
	sortedNodes, _ := topo.Sort(p.Graph)
	for i := len(sortedNodes) - 1; i >= 0; i-- {
		op := sortedNodes[i].(OperatorNode).Operator()

		scopedPersister := operator.NewScopedPersister(op.ID(), persister)
		op.Logger().Debug("Starting operator")
		if err := op.Start(scopedPersister); err != nil {
			return err
		}
		op.Logger().Debug("Started operator")
	}

	return nil
}

func (p *DirectedPipeline) stop() error {
	var err error
	sortedNodes, _ := topo.Sort(p.Graph)
	for _, node := range sortedNodes {
		operator := node.(OperatorNode).Operator()
		operator.Logger().Debug("Stopping operator")
		if opErr := operator.Stop(); opErr != nil {
			err = multierr.Append(err, opErr)
		}
		operator.Logger().Debug("Stopped operator")
	}
	return err
}

// Render will render the pipeline as a dot graph
func (p *DirectedPipeline) Render() ([]byte, error) {
	return dot.Marshal(p.Graph, "G", "", " ")
}

// Operators returns a slice of operators that make up the pipeline graph
func (p *DirectedPipeline) Operators() []operator.Operator {
	var operators []operator.Operator
	if nodes, err := topo.Sort(p.Graph); err == nil {
		for _, node := range nodes {
			operators = append(operators, node.(OperatorNode).Operator())
		}
		return operators
	}

	// If for some unexpected reason an Unorderable error is returned,
	// when using topo.Sort, return the list without ordering
	nodes := p.Graph.Nodes()
	for nodes.Next() {
		operators = append(operators, nodes.Node().(OperatorNode).Operator())
	}
	return operators
}

// addNodes will add operators as nodes to the supplied graph.
func addNodes(graph *simple.DirectedGraph, operators []operator.Operator) error {
	for _, operator := range operators {
		operatorNode := createOperatorNode(operator)
		if graph.Node(operatorNode.ID()) != nil {
			return stanzaerrors.NewError(
				fmt.Sprintf("operator with id '%s' already exists in pipeline", operatorNode.Operator().ID()),
				"ensure that each operator has a unique `type` or `id`",
			)
		}

		graph.AddNode(operatorNode)
	}
	return nil
}

// connectNodes will connect the nodes in the supplied graph.
func connectNodes(graph *simple.DirectedGraph) error {
	nodes := graph.Nodes()
	for nodes.Next() {
		node := nodes.Node().(OperatorNode)
		if err := connectNode(graph, node); err != nil {
			return err
		}
	}

	if _, err := topo.Sort(graph); err != nil {
		var topoErr topo.Unorderable
		errors.As(err, &topoErr)
		return stanzaerrors.NewError(
			"pipeline has a circular dependency",
			"ensure that all operators are connected in a straight, acyclic line",
			"cycles", unorderableToCycles(topoErr),
		)
	}

	return nil
}

// connectNode will connect a node to its outputs in the supplied graph.
func connectNode(graph *simple.DirectedGraph, inputNode OperatorNode) error {
	for outputOperatorID, outputNodeID := range inputNode.OutputIDs() {
		if graph.Node(outputNodeID) == nil {
			return stanzaerrors.NewError(
				"operators cannot be connected, because the output does not exist in the pipeline",
				"ensure that the output operator is defined",
				"input_operator", inputNode.Operator().ID(),
				"output_operator", outputOperatorID,
			)
		}

		outputNode := graph.Node(outputNodeID).(OperatorNode)
		if !outputNode.Operator().CanProcess() {
			return stanzaerrors.NewError(
				"operators cannot be connected, because the output operator can not process logs",
				"ensure that the output operator can process logs (like a parser or destination)",
				"input_operator", inputNode.Operator().ID(),
				"output_operator", outputOperatorID,
			)
		}

		if graph.HasEdgeFromTo(inputNode.ID(), outputNodeID) {
			return stanzaerrors.NewError(
				"operators cannot be connected, because a connection already exists",
				"ensure that only a single connection exists between the two operators",
				"input_operator", inputNode.Operator().ID(),
				"output_operator", outputOperatorID,
			)
		}

		edge := graph.NewEdge(inputNode, outputNode)
		graph.SetEdge(edge)
	}

	return nil
}

// setOperatorOutputs will set the outputs on operators that can output.
func setOperatorOutputs(operators []operator.Operator) error {
	for _, operator := range operators {
		if !operator.CanOutput() {
			continue
		}

		if err := operator.SetOutputs(operators); err != nil {
			return stanzaerrors.WithDetails(err, "operator_id", operator.ID())
		}
	}
	return nil
}

// NewDirectedPipeline creates a new directed pipeline
func NewDirectedPipeline(operators []operator.Operator) (*DirectedPipeline, error) {
	if err := setOperatorOutputs(operators); err != nil {
		return nil, err
	}

	graph := simple.NewDirectedGraph()
	if err := addNodes(graph, operators); err != nil {
		return nil, err
	}

	if err := connectNodes(graph); err != nil {
		return nil, err
	}

	return &DirectedPipeline{Graph: graph}, nil
}

func unorderableToCycles(err topo.Unorderable) string {
	var cycles strings.Builder
	for i, cycle := range err {
		if i != 0 {
			cycles.WriteByte(',')
		}
		cycles.WriteByte('(')
		for _, node := range cycle {
			cycles.WriteString(node.(OperatorNode).operator.ID())
			cycles.Write([]byte(` -> `))
		}
		cycles.WriteString(cycle[0].(OperatorNode).operator.ID())
		cycles.WriteByte(')')
	}
	return cycles.String()
}
