// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

const (
	stratifiedDefaultHashSalt = "default-hash-seed"
)

type StratifiedProbabilisticSampler struct {
	logger               *zap.Logger
	threshold            uint64
	hashSalt             string
	mu                   sync.Mutex
	traceTrajectoryCount map[string]int
}

type Node struct {
	Service   string
	Operation string
}

type Edge struct {
	From Node
	To   Node
}

var _ PolicyEvaluator = (*StratifiedProbabilisticSampler)(nil)

// NewStratifiedProbabilisticSampler creates a policy evaluator that samples a percentage of traces.
func NewStratifiedProbabilisticSampler(settings component.TelemetrySettings, hashSalt string, samplingPercentage float64) PolicyEvaluator {
	if hashSalt == "" {
		hashSalt = stratifiedDefaultHashSalt
	}

	return &StratifiedProbabilisticSampler{
		logger: settings.Logger,
		// calculate threshold once
		threshold:            stratifiedCalculateThreshold(samplingPercentage / 100),
		hashSalt:             hashSalt,
		traceTrajectoryCount: make(map[string]int),
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (s *StratifiedProbabilisticSampler) Evaluate(_ context.Context, traceID pcommon.TraceID, traceData *TraceData) (Decision, error) {
	s.logger.Debug("Evaluating spans in stratified probabilistic filter")
	hash := s.getTraceTrajectoryHash(traceData)
	s.logger.Debug("Graph representation received", zap.String("Trace Identifier", traceID.String()), zap.String("Trace Hash", hash))

	var count int
	var exists bool

	s.mu.Lock()
	defer s.mu.Unlock()

	count, exists = s.traceTrajectoryCount[hash]

	if !exists {
		// Trajectory is new for the interval
		s.traceTrajectoryCount[hash] = 1
		s.logger.Debug("New Trajectory", zap.String("Trace Hash", hash), zap.String("Check for Trace Identifier", traceID.String()), zap.Int("TraceCount", count))
		return Sampled, nil
	}
	// Trajectory exist. Update the count
	s.traceTrajectoryCount[hash] = count + 1
	s.logger.Debug("Trajectory exists", zap.String("Trace Hash", hash), zap.String("Check for Trace Identifier", traceID.String()), zap.Int("TraceCount", count))

	// Fallback to probabilistic sampling if the trajectory is seen before

	if stratifiedHashTraceID(s.hashSalt, traceID[:]) <= s.threshold {
		return Sampled, nil
	}

	return NotSampled, nil
}

func (s *StratifiedProbabilisticSampler) ResetWindow() {
	s.logger.Debug("Resetting Evaluation Window")
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Debug("Trace trajectory count map", zap.Any("traceTrajectoryCount", s.traceTrajectoryCount))
	s.traceTrajectoryCount = make(map[string]int)
}

func (s *StratifiedProbabilisticSampler) getTraceTrajectoryHash(traceData *TraceData) string {
	nodes, edges := s.getTraceSpanDetails(traceData)

	// Build adjacency list and in-degree map
	adj := make(map[Node][]Node)
	inDegree := make(map[Node]int)
	allNodes := make(map[Node]struct{})

	for _, node := range nodes {
		allNodes[node] = struct{}{}
		if _, exists := inDegree[node]; !exists {
			inDegree[node] = 0
		}
	}

	for _, edge := range edges {
		if !isEmptyNode(edge.From) {
			adj[edge.From] = append(adj[edge.From], edge.To)
			inDegree[edge.To]++
			allNodes[edge.From] = struct{}{}
		} else {
			// Edge from empty node indicates a root span
			if _, exists := inDegree[edge.To]; !exists {
				inDegree[edge.To] = 0
			}
		}
		allNodes[edge.To] = struct{}{}
	}

	// Canonical topological sort
	// var sorted []Node
	zeroInDegree := []Node{}

	for node := range allNodes {
		if inDegree[node] == 0 {
			zeroInDegree = append(zeroInDegree, node)
		}
	}
	sort.Slice(zeroInDegree, func(i, j int) bool {
		return compareNodes(zeroInDegree[i], zeroInDegree[j]) < 0
	})

	for len(zeroInDegree) > 0 {
		node := zeroInDegree[0]
		zeroInDegree = zeroInDegree[1:]
		// sorted = append(sorted, node)

		children := adj[node]
		sort.Slice(children, func(i, j int) bool {
			return compareNodes(children[i], children[j]) < 0
		})

		for _, child := range children {
			inDegree[child]--
			if inDegree[child] == 0 {
				zeroInDegree = append(zeroInDegree, child)
			}
		}
		sort.Slice(zeroInDegree, func(i, j int) bool {
			return compareNodes(zeroInDegree[i], zeroInDegree[j]) < 0
		})
	}

	// Serialize edges deterministically with quoting
	edgeStrs := make([]string, len(edges))
	for i, e := range edges {
		fromLabel := fmt.Sprintf("%s:%s", e.From.Service, e.From.Operation)
		toLabel := fmt.Sprintf("%s:%s", e.To.Service, e.To.Operation)
		edgeStrs[i] = strconv.Quote(fromLabel) + "->" + strconv.Quote(toLabel)
	}
	sort.Strings(edgeStrs)
	edgesStr := fmt.Sprintf("[%s]", joinWithComma(edgeStrs))

	// Construct graph representation and hash it
	graphRepr := edgesStr
	h := sha256.Sum256([]byte(graphRepr))
	hash := hex.EncodeToString(h[:])

	// Log the canonical representation and hash
	s.logger.Debug("Graph representation and hash",
		zap.String("Graph representation", graphRepr),
		zap.String("Trace Hash", hash),
	)

	return hash
}

func isEmptyNode(n Node) bool {
	return n.Service == "" && n.Operation == ""
}

// compareNodes ensures deterministic sorting of Node structs
func compareNodes(a, b Node) int {
	if a.Service != b.Service {
		return compareString(a.Service, b.Service)
	}
	return compareString(a.Operation, b.Operation)
}

func compareString(a, b string) int {
	if a < b {
		return -1
	} else if a > b {
		return 1
	}
	return 0
}

func joinWithComma(items []string) string {
	return strings.Join(items, ",")
}

func (s *StratifiedProbabilisticSampler) getTraceSpanDetails(traceData *TraceData) ([]Node, []Edge) {
	s.logger.Debug("Extracting span details for the trace")
	traceData.Lock()
	defer traceData.Unlock()

	batches := traceData.ReceivedBatches

	nodeSet := make(map[Node]struct{})
	spanIDToNode := make(map[string]Node)
	spanIDToParentID := make(map[string]string)

	// First pass: build spanID to Node mapping and collect span -> parentSpan relationships
	for i := 0; i < batches.ResourceSpans().Len(); i++ {
		rs := batches.ResourceSpans().At(i)
		resource := rs.Resource()

		var serviceName string
		if svcAttr, ok := resource.Attributes().Get("service.name"); ok {
			serviceName = svcAttr.AsString()
		}

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				spanID := span.SpanID().String()
				parentSpanID := span.ParentSpanID().String()
				operationName := span.Name()
				// traceID := span.TraceID().String()

				node := Node{Service: serviceName, Operation: operationName}
				nodeSet[node] = struct{}{}
				spanIDToNode[spanID] = node
				spanIDToParentID[spanID] = parentSpanID
			}
		}
	}

	// Second pass: build edges from parent to child
	edges := []Edge{}
	for childSpanID, parentSpanID := range spanIDToParentID {
		childNode := spanIDToNode[childSpanID]
		var parentNode Node
		if parentSpanID == "" {
			// Root span
			parentNode = Node{Service: "", Operation: ""}
		} else {
			parentNode = spanIDToNode[parentSpanID]
		}
		newEdge := Edge{From: parentNode, To: childNode}
		edges = insertSortedEdge(edges, newEdge)
	}

	// Convert node set to slice
	nodes := make([]Node, 0, len(nodeSet))
	for node := range nodeSet {
		nodes = append(nodes, node)
	}

	return nodes, edges
}

// Helper function to insert an edge in sorted order
func insertSortedEdge(edges []Edge, newEdge Edge) []Edge {
	// Find the insertion point by comparing the edges (using lexicographical order)
	idx := sort.Search(len(edges), func(i int) bool {
		return compareEdges(edges[i], newEdge) >= 0
	})

	// Insert the new edge at the found position
	edges = append(edges[:idx], append([]Edge{newEdge}, edges[idx:]...)...)
	return edges
}

// Helper function to compare two edges lexicographically
func compareEdges(e1, e2 Edge) int {
	// Compare "From" (parent node)
	if e1.From.Service != e2.From.Service {
		if e1.From.Service < e2.From.Service {
			return -1
		}
		return 1
	}
	if e1.From.Operation != e2.From.Operation {
		if e1.From.Operation < e2.From.Operation {
			return -1
		}
		return 1
	}

	// Compare "To" (child node)
	if e1.To.Service != e2.To.Service {
		if e1.To.Service < e2.To.Service {
			return -1
		}
		return 1
	}
	if e1.To.Operation != e2.To.Operation {
		if e1.To.Operation < e2.To.Operation {
			return -1
		}
		return 1
	}

	return 0 // They are equal
}

// calculateThreshold converts a ratio into a value between 0 and MaxUint64
func stratifiedCalculateThreshold(ratio float64) uint64 {
	// Use big.Float and big.Int to calculate threshold because directly convert
	// math.MaxUint64 to float64 will cause digits/bits to be cut off if the converted value
	// doesn't fit into bits that are used to store digits for float64 in Golang
	boundary := new(big.Float).SetInt(new(big.Int).SetUint64(math.MaxUint64))
	res, _ := boundary.Mul(boundary, big.NewFloat(ratio)).Uint64()
	return res
}

// hashTraceID creates a hash using the FNV-1a algorithm.
func stratifiedHashTraceID(salt string, b []byte) uint64 {
	hasher := fnv.New64a()
	// the implementation fnv.Write() never returns an error, see hash/fnv/fnv.go
	_, _ = hasher.Write([]byte(salt))
	_, _ = hasher.Write(b)
	return hasher.Sum64()
}
