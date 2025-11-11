// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ExecutionPlanLogger handles logging of parsed execution plan data to New Relic
type ExecutionPlanLogger struct {
	logger *zap.Logger
}

// NewExecutionPlanLogger creates a new execution plan logger configured for New Relic
func NewExecutionPlanLogger() *ExecutionPlanLogger {
	// Configure logger for structured JSON output suitable for New Relic
	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.InfoLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding: "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, _ := config.Build()
	return &ExecutionPlanLogger{
		logger: logger,
	}
}

// LogExecutionPlanAnalysis logs the complete execution plan analysis as structured JSON
func (epl *ExecutionPlanLogger) LogExecutionPlanAnalysis(analysis *ExecutionPlanAnalysis) {
	if analysis == nil {
		epl.logger.Warn("Attempted to log nil execution plan analysis")
		return
	}

	// Create the main log entry with plan summary
	planSummary := map[string]interface{}{
		"event_type":      "SQLServerExecutionPlan",
		"query_id":        analysis.QueryID,
		"plan_handle":     analysis.PlanHandle,
		"sql_text":        analysis.SQLText,
		"total_cost":      analysis.TotalCost,
		"compile_time":    analysis.CompileTime,
		"compile_cpu":     analysis.CompileCPU,
		"compile_memory":  analysis.CompileMemory,
		"collection_time": analysis.CollectionTime,
		"total_operators": len(analysis.Nodes),
	}

	// Add aggregated operator statistics
	planSummary["operator_summary"] = epl.generateOperatorSummary(analysis.Nodes)

	// Add performance insights
	planSummary["performance_insights"] = epl.generatePerformanceInsights(analysis.Nodes)

	epl.logger.Info("SQL Server Execution Plan Analysis",
		zap.Any("execution_plan_summary", planSummary),
		zap.String("component", "newrelicsqlserverreceiver"),
		zap.String("log_type", "execution_plan_analysis"))

	// Log each operator as a separate detailed entry
	for _, node := range analysis.Nodes {
		epl.LogExecutionPlanNode(&node)
	}
}

// LogExecutionPlanNode logs an individual execution plan node with full details
func (epl *ExecutionPlanLogger) LogExecutionPlanNode(node *ExecutionPlanNode) {
	if node == nil {
		epl.logger.Warn("Attempted to log nil execution plan node")
		return
	}

	// Create structured log entry for the operator
	operatorData := map[string]interface{}{
		"event_type":               "SQLServerExecutionPlanNode",
		"query_id":                 node.QueryID,
		"plan_handle":              node.PlanHandle,
		"node_id":                  node.NodeID,
		"sql_text":                 node.SQLText,
		"physical_op":              node.PhysicalOp,
		"logical_op":               node.LogicalOp,
		"estimate_rows":            node.EstimateRows,
		"estimate_io":              node.EstimateIO,
		"estimate_cpu":             node.EstimateCPU,
		"avg_row_size":             node.AvgRowSize,
		"total_subtree_cost":       node.TotalSubtreeCost,
		"estimated_operator_cost":  node.EstimatedOperatorCost,
		"estimated_execution_mode": node.EstimatedExecutionMode,
		"granted_memory_kb":        node.GrantedMemoryKb,
		"spill_occurred":           node.SpillOccurred,
		"no_join_predicate":        node.NoJoinPredicate,
		"total_worker_time":        node.TotalWorkerTime,
		"total_elapsed_time":       node.TotalElapsedTime,
		"total_logical_reads":      node.TotalLogicalReads,
		"total_logical_writes":     node.TotalLogicalWrites,
		"execution_count":          node.ExecutionCount,
		"avg_elapsed_time_ms":      node.AvgElapsedTimeMs,
		"collection_timestamp":     node.CollectionTimestamp,
		"last_execution_time":      node.LastExecutionTime,
	}

	// Add operator category and performance classification
	operatorData["operator_category"] = epl.categorizeOperator(node.PhysicalOp)
	operatorData["performance_impact"] = epl.assessPerformanceImpact(node)
	operatorData["optimization_hints"] = epl.generateOptimizationHints(node)

	epl.logger.Info("SQL Server Execution Plan Operator",
		zap.Any("execution_plan_node", operatorData),
		zap.String("component", "newrelicsqlserverreceiver"),
		zap.String("log_type", "execution_plan_node"),
		zap.String("operator_type", node.PhysicalOp),
		zap.Int("node_id", node.NodeID))
}

// generateOperatorSummary creates a summary of operators in the execution plan
func (epl *ExecutionPlanLogger) generateOperatorSummary(nodes []ExecutionPlanNode) map[string]interface{} {
	operatorCounts := make(map[string]int)
	totalCost := 0.0
	totalRows := 0.0
	hasSpills := false
	hasMissingPredicates := false

	for _, node := range nodes {
		operatorCounts[node.PhysicalOp]++
		totalCost += node.TotalSubtreeCost
		totalRows += node.EstimateRows

		if node.SpillOccurred {
			hasSpills = true
		}
		if node.NoJoinPredicate {
			hasMissingPredicates = true
		}
	}

	return map[string]interface{}{
		"operator_counts":         operatorCounts,
		"total_estimated_cost":    totalCost,
		"total_estimated_rows":    totalRows,
		"has_memory_spills":       hasSpills,
		"has_missing_predicates":  hasMissingPredicates,
		"most_expensive_operator": epl.findMostExpensiveOperator(nodes),
	}
}

// generatePerformanceInsights provides performance analysis of the execution plan
func (epl *ExecutionPlanLogger) generatePerformanceInsights(nodes []ExecutionPlanNode) map[string]interface{} {
	insights := map[string]interface{}{
		"potential_issues":           []string{},
		"optimization_opportunities": []string{},
		"performance_warnings":       []string{},
	}

	issues := []string{}
	opportunities := []string{}
	warnings := []string{}

	for _, node := range nodes {
		// Check for potential issues
		if node.SpillOccurred {
			issues = append(issues, fmt.Sprintf("Memory spill detected in %s operator (Node %d)", node.PhysicalOp, node.NodeID))
		}

		if node.NoJoinPredicate {
			issues = append(issues, fmt.Sprintf("Missing join predicate in %s operator (Node %d)", node.PhysicalOp, node.NodeID))
		}

		// Check for high-cost operations
		if node.TotalSubtreeCost > 50.0 {
			warnings = append(warnings, fmt.Sprintf("High-cost operation: %s (Cost: %.2f, Node %d)", node.PhysicalOp, node.TotalSubtreeCost, node.NodeID))
		}

		// Check for optimization opportunities
		if node.PhysicalOp == "Clustered Index Scan" && node.EstimateRows > 100000 {
			opportunities = append(opportunities, fmt.Sprintf("Consider index optimization for large scan (Node %d, %d estimated rows)", node.NodeID, int64(node.EstimateRows)))
		}

		if node.PhysicalOp == "Key Lookup" {
			opportunities = append(opportunities, fmt.Sprintf("Key lookup detected - consider covering index (Node %d)", node.NodeID))
		}
	}

	insights["potential_issues"] = issues
	insights["optimization_opportunities"] = opportunities
	insights["performance_warnings"] = warnings

	return insights
}

// categorizeOperator categorizes the physical operator type
func (epl *ExecutionPlanLogger) categorizeOperator(physicalOp string) string {
	categories := map[string]string{
		"Clustered Index Scan": "Data Access",
		"Index Scan":           "Data Access",
		"Index Seek":           "Data Access",
		"Key Lookup":           "Data Access",
		"Table Scan":           "Data Access",
		"Nested Loops":         "Join",
		"Hash Match":           "Join",
		"Merge Join":           "Join",
		"Sort":                 "Sort/Aggregate",
		"Stream Aggregate":     "Sort/Aggregate",
		"Hash Aggregate":       "Sort/Aggregate",
		"Compute Scalar":       "Computation",
		"Filter":               "Filter",
		"Parallelism":          "Parallelism",
		"Sequence Project":     "Projection",
	}

	if category, exists := categories[physicalOp]; exists {
		return category
	}
	return "Other"
}

// assessPerformanceImpact assesses the performance impact of an operator
func (epl *ExecutionPlanLogger) assessPerformanceImpact(node *ExecutionPlanNode) string {
	if node.TotalSubtreeCost > 100.0 {
		return "High"
	}
	if node.TotalSubtreeCost > 10.0 {
		return "Medium"
	}
	return "Low"
}

// generateOptimizationHints provides optimization suggestions for the operator
func (epl *ExecutionPlanLogger) generateOptimizationHints(node *ExecutionPlanNode) []string {
	hints := []string{}

	if node.SpillOccurred {
		hints = append(hints, "Consider increasing server memory or optimizing query to reduce memory usage")
	}

	if node.NoJoinPredicate {
		hints = append(hints, "Add appropriate WHERE clause or JOIN conditions to avoid Cartesian products")
	}

	if node.PhysicalOp == "Key Lookup" {
		hints = append(hints, "Create covering index to eliminate key lookups")
	}

	if node.PhysicalOp == "Table Scan" || node.PhysicalOp == "Clustered Index Scan" {
		if node.EstimateRows > 10000 {
			hints = append(hints, "Consider adding selective indexes or improving WHERE clause selectivity")
		}
	}

	if node.TotalSubtreeCost > 50.0 {
		hints = append(hints, "High-cost operation - review query logic and indexing strategy")
	}

	return hints
}

// findMostExpensiveOperator identifies the most expensive operator in the plan
func (epl *ExecutionPlanLogger) findMostExpensiveOperator(nodes []ExecutionPlanNode) map[string]interface{} {
	if len(nodes) == 0 {
		return nil
	}

	maxCost := 0.0
	var expensiveNode *ExecutionPlanNode

	for i, node := range nodes {
		if node.TotalSubtreeCost > maxCost {
			maxCost = node.TotalSubtreeCost
			expensiveNode = &nodes[i]
		}
	}

	if expensiveNode == nil {
		return nil
	}

	return map[string]interface{}{
		"node_id":     expensiveNode.NodeID,
		"physical_op": expensiveNode.PhysicalOp,
		"cost":        expensiveNode.TotalSubtreeCost,
		"percentage":  (expensiveNode.TotalSubtreeCost / maxCost) * 100,
	}
}

// LogExecutionPlanJSON logs execution plan data as raw JSON for external processing
func (epl *ExecutionPlanLogger) LogExecutionPlanJSON(analysis *ExecutionPlanAnalysis) error {
	if analysis == nil {
		return fmt.Errorf("cannot log nil execution plan analysis")
	}

	// Convert to JSON for raw logging
	jsonData, err := json.Marshal(analysis)
	if err != nil {
		return fmt.Errorf("failed to marshal execution plan to JSON: %w", err)
	}

	// Log as structured JSON for New Relic ingestion
	epl.logger.Info("SQL Server Execution Plan JSON",
		zap.String("execution_plan_json", string(jsonData)),
		zap.String("query_id", analysis.QueryID),
		zap.String("plan_handle", analysis.PlanHandle),
		zap.String("component", "newrelicsqlserverreceiver"),
		zap.String("log_type", "execution_plan_json"),
		zap.Int("node_count", len(analysis.Nodes)))

	return nil
}
