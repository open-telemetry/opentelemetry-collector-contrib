// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// XML Structure for SQL Server Execution Plans
// Based on Microsoft's showplan XML schema

// ShowPlanXML represents the root element of SQL Server execution plan XML
type ShowPlanXML struct {
	XMLName       xml.Name        `xml:"ShowPlanXML"`
	Version       string          `xml:"Version,attr"`
	Build         string          `xml:"Build,attr"`
	BatchSequence []BatchSequence `xml:"BatchSequence"`
}

// BatchSequence represents a batch of SQL statements
type BatchSequence struct {
	XMLName xml.Name `xml:"BatchSequence"`
	Batch   []Batch  `xml:"Batch"`
}

// Batch represents a single batch execution
type Batch struct {
	XMLName    xml.Name     `xml:"Batch"`
	Statements []StmtSimple `xml:"Statements>StmtSimple"`
}

// StmtSimple represents a simple SQL statement
type StmtSimple struct {
	XMLName                           xml.Name  `xml:"StmtSimple"`
	StatementText                     string    `xml:"StatementText,attr"`
	StatementId                       int       `xml:"StatementId,attr"`
	StatementCompId                   int       `xml:"StatementCompId,attr"`
	StatementType                     string    `xml:"StatementType,attr"`
	RetrievedFromCache                string    `xml:"RetrievedFromCache,attr"`
	StatementSubTreeCost              string    `xml:"StatementSubTreeCost,attr"`
	StatementEstRows                  string    `xml:"StatementEstRows,attr"`
	SecurityPolicyApplied             string    `xml:"SecurityPolicyApplied,attr"`
	StatementOptmLevel                string    `xml:"StatementOptmLevel,attr"`
	QueryHash                         string    `xml:"QueryHash,attr"`
	QueryPlanHash                     string    `xml:"QueryPlanHash,attr"`
	CardinalityEstimationModelVersion string    `xml:"CardinalityEstimationModelVersion,attr"`
	QueryPlan                         QueryPlan `xml:"QueryPlan"`
}

// QueryPlan represents the execution plan details
type QueryPlan struct {
	XMLName               xml.Name        `xml:"QueryPlan"`
	DegreeOfParallelism   string          `xml:"DegreeOfParallelism,attr"`
	MemoryGrant           string          `xml:"MemoryGrant,attr"`
	CachedPlanSize        string          `xml:"CachedPlanSize,attr"`
	CompileTime           string          `xml:"CompileTime,attr"`
	CompileCPU            string          `xml:"CompileCPU,attr"`
	CompileMemory         string          `xml:"CompileMemory,attr"`
	NonParallelPlanReason string          `xml:"NonParallelPlanReason,attr"`
	RelOp                 RelOp           `xml:"RelOp"`
	ParameterList         *ParameterList  `xml:"ParameterList,omitempty"`
	QueryTimeStats        *QueryTimeStats `xml:"QueryTimeStats,omitempty"`
}

// RelOp represents a relational operator in the execution plan
type RelOp struct {
	XMLName                   xml.Name `xml:"RelOp"`
	NodeId                    int      `xml:"NodeId,attr"`
	PhysicalOp                string   `xml:"PhysicalOp,attr"`
	LogicalOp                 string   `xml:"LogicalOp,attr"`
	EstimateRows              string   `xml:"EstimateRows,attr"`
	EstimateIO                string   `xml:"EstimateIO,attr"`
	EstimateCPU               string   `xml:"EstimateCPU,attr"`
	AvgRowSize                string   `xml:"AvgRowSize,attr"`
	EstimatedTotalSubtreeCost string   `xml:"EstimatedTotalSubtreeCost,attr"`
	Parallel                  string   `xml:"Parallel,attr"`
	EstimateRebinds           string   `xml:"EstimateRebinds,attr"`
	EstimateRewinds           string   `xml:"EstimateRewinds,attr"`
	EstimatedExecutionMode    string   `xml:"EstimatedExecutionMode,attr"`

	// Child operators
	RelOp []RelOp `xml:"RelOp"`

	// Operator-specific details
	RunTimeInformation *RunTimeInformation `xml:"RunTimeInformation,omitempty"`
	MemoryFractions    *MemoryFractions    `xml:"MemoryFractions,omitempty"`
	IndexScan          *IndexScan          `xml:"IndexScan,omitempty"`
	NestedLoops        *NestedLoops        `xml:"NestedLoops,omitempty"`
	Hash               *Hash               `xml:"Hash,omitempty"`
	Sort               *Sort               `xml:"Sort,omitempty"`
}

// RunTimeInformation contains actual execution statistics
type RunTimeInformation struct {
	XMLName           xml.Name                   `xml:"RunTimeInformation"`
	CountersPerThread []RunTimeCountersPerThread `xml:"RunTimeCountersPerThread"`
}

// RunTimeCountersPerThread contains per-thread execution counters
type RunTimeCountersPerThread struct {
	XMLName                xml.Name `xml:"RunTimeCountersPerThread"`
	Thread                 int      `xml:"Thread,attr"`
	ActualRows             string   `xml:"ActualRows,attr"`
	ActualEndOfScans       string   `xml:"ActualEndOfScans,attr"`
	ActualExecutions       string   `xml:"ActualExecutions,attr"`
	ActualExecutionMode    string   `xml:"ActualExecutionMode,attr"`
	ActualElapsedms        string   `xml:"ActualElapsedms,attr"`
	ActualCPUms            string   `xml:"ActualCPUms,attr"`
	ActualScans            string   `xml:"ActualScans,attr"`
	ActualLogicalReads     string   `xml:"ActualLogicalReads,attr"`
	ActualPhysicalReads    string   `xml:"ActualPhysicalReads,attr"`
	ActualReadAheads       string   `xml:"ActualReadAheads,attr"`
	ActualLobLogicalReads  string   `xml:"ActualLobLogicalReads,attr"`
	ActualLobPhysicalReads string   `xml:"ActualLobPhysicalReads,attr"`
	ActualLobReadAheads    string   `xml:"ActualLobReadAheads,attr"`
}

// MemoryFractions contains memory grant information
type MemoryFractions struct {
	XMLName xml.Name `xml:"MemoryFractions"`
	Input   string   `xml:"Input,attr"`
	Output  string   `xml:"Output,attr"`
}

// IndexScan contains index scan operation details
type IndexScan struct {
	XMLName       xml.Name `xml:"IndexScan"`
	Ordered       string   `xml:"Ordered,attr"`
	ScanDirection string   `xml:"ScanDirection,attr"`
	ForcedIndex   string   `xml:"ForcedIndex,attr"`
	ForceSeek     string   `xml:"ForceSeek,attr"`
}

// NestedLoops contains nested loop join details
type NestedLoops struct {
	XMLName               xml.Name `xml:"NestedLoops"`
	Optimized             string   `xml:"Optimized,attr"`
	WithUnorderedPrefetch string   `xml:"WithUnorderedPrefetch,attr"`
}

// Hash contains hash operation details
type Hash struct {
	XMLName       xml.Name `xml:"Hash"`
	BitmapCreator string   `xml:"BitmapCreator,attr"`
}

// Sort contains sort operation details
type Sort struct {
	XMLName  xml.Name `xml:"Sort"`
	Distinct string   `xml:"Distinct,attr"`
}

// ParameterList contains query parameters
type ParameterList struct {
	XMLName         xml.Name          `xml:"ParameterList"`
	ColumnReference []ColumnReference `xml:"ColumnReference"`
}

// ColumnReference represents a column or parameter reference
type ColumnReference struct {
	XMLName                xml.Name `xml:"ColumnReference"`
	Column                 string   `xml:"Column,attr"`
	ParameterCompiledValue string   `xml:"ParameterCompiledValue,attr"`
	ParameterRuntimeValue  string   `xml:"ParameterRuntimeValue,attr"`
}

// QueryTimeStats contains query execution timing statistics
type QueryTimeStats struct {
	XMLName     xml.Name `xml:"QueryTimeStats"`
	ElapsedTime string   `xml:"ElapsedTime,attr"`
	CpuTime     string   `xml:"CpuTime,attr"`
}

// ParseExecutionPlanXML parses SQL Server execution plan XML into structured data
func ParseExecutionPlanXML(planXML string, queryID, planHandle string) (*ExecutionPlanAnalysis, error) {
	if planXML == "" {
		return nil, fmt.Errorf("empty execution plan XML")
	}

	var showPlan ShowPlanXML
	err := xml.Unmarshal([]byte(planXML), &showPlan)
	if err != nil {
		return nil, fmt.Errorf("failed to parse execution plan XML: %w", err)
	}

	analysis := &ExecutionPlanAnalysis{
		QueryID:        queryID,
		PlanHandle:     planHandle,
		Nodes:          []ExecutionPlanNode{},
		CollectionTime: time.Now().UTC().Format(time.RFC3339),
	}

	// Extract SQL text and plan-level information
	if len(showPlan.BatchSequence) > 0 && len(showPlan.BatchSequence[0].Batch) > 0 {
		batch := showPlan.BatchSequence[0].Batch[0]
		if len(batch.Statements) > 0 {
			stmt := batch.Statements[0]
			analysis.SQLText = stmt.StatementText

			// Parse plan-level costs and timing
			if cost, err := strconv.ParseFloat(stmt.StatementSubTreeCost, 64); err == nil {
				analysis.TotalCost = cost
			}

			// Parse query plan details
			queryPlan := stmt.QueryPlan
			analysis.CompileTime = queryPlan.CompileTime
			if cpu, err := strconv.ParseInt(queryPlan.CompileCPU, 10, 64); err == nil {
				analysis.CompileCPU = cpu
			}
			if memory, err := strconv.ParseInt(queryPlan.CompileMemory, 10, 64); err == nil {
				analysis.CompileMemory = memory
			}

			// Parse operators recursively
			nodeID := 0
			parseRelOpRecursively(&queryPlan.RelOp, analysis, queryID, planHandle, &nodeID)
		}
	}

	return analysis, nil
}

// parseRelOpRecursively recursively parses relational operators in the execution plan
func parseRelOpRecursively(relOp *RelOp, analysis *ExecutionPlanAnalysis, queryID, planHandle string, nodeID *int) {
	if relOp == nil {
		return
	}

	*nodeID++
	node := ExecutionPlanNode{
		QueryID:                queryID,
		PlanHandle:             planHandle,
		NodeID:                 relOp.NodeId,
		SQLText:                analysis.SQLText,
		PhysicalOp:             relOp.PhysicalOp,
		LogicalOp:              relOp.LogicalOp,
		EstimatedExecutionMode: relOp.EstimatedExecutionMode,
		CollectionTimestamp:    analysis.CollectionTime,
	}

	// Parse numeric values safely
	if val, err := strconv.ParseFloat(relOp.EstimateRows, 64); err == nil {
		node.EstimateRows = val
	}
	if val, err := strconv.ParseFloat(relOp.EstimateIO, 64); err == nil {
		node.EstimateIO = val
	}
	if val, err := strconv.ParseFloat(relOp.EstimateCPU, 64); err == nil {
		node.EstimateCPU = val
	}
	if val, err := strconv.ParseFloat(relOp.AvgRowSize, 64); err == nil {
		node.AvgRowSize = val
	}
	if val, err := strconv.ParseFloat(relOp.EstimatedTotalSubtreeCost, 64); err == nil {
		node.TotalSubtreeCost = val
		node.EstimatedOperatorCost = val // Operator cost is part of subtree cost
	}

	// Parse runtime information if available
	if relOp.RunTimeInformation != nil && len(relOp.RunTimeInformation.CountersPerThread) > 0 {
		counter := relOp.RunTimeInformation.CountersPerThread[0] // Take first thread's data

		if val, err := strconv.ParseFloat(counter.ActualElapsedms, 64); err == nil {
			node.TotalElapsedTime = val
			node.AvgElapsedTimeMs = val
		}
		if val, err := strconv.ParseFloat(counter.ActualCPUms, 64); err == nil {
			node.TotalWorkerTime = val
		}
		if val, err := strconv.ParseInt(counter.ActualLogicalReads, 10, 64); err == nil {
			node.TotalLogicalReads = val
		}
		if val, err := strconv.ParseInt(counter.ActualExecutions, 10, 64); err == nil {
			node.ExecutionCount = val
		}
	}

	// Check for performance issues
	node.SpillOccurred = checkForSpill(relOp)
	node.NoJoinPredicate = checkForMissingJoinPredicate(relOp)

	// Set current timestamp
	node.LastExecutionTime = time.Now().UTC().Format(time.RFC3339)

	analysis.Nodes = append(analysis.Nodes, node)

	// Recursively parse child operators
	for i := range relOp.RelOp {
		parseRelOpRecursively(&relOp.RelOp[i], analysis, queryID, planHandle, nodeID)
	}
}

// checkForSpill checks if the operator experienced memory spills
func checkForSpill(relOp *RelOp) bool {
	// Look for spill indicators in sort, hash, and other memory-intensive operators
	opType := strings.ToLower(relOp.PhysicalOp)

	// Common operators that can spill
	spillOperators := []string{"sort", "hash match", "hash join", "stream aggregate", "window spool"}
	for _, spillOp := range spillOperators {
		if strings.Contains(opType, spillOp) {
			// In a real implementation, you'd check for specific spill warnings or memory grants
			// For now, we'll use a heuristic based on cost
			if cost, err := strconv.ParseFloat(relOp.EstimatedTotalSubtreeCost, 64); err == nil && cost > 10.0 {
				return true
			}
		}
	}
	return false
}

// checkForMissingJoinPredicate checks if a join operation is missing proper predicates
func checkForMissingJoinPredicate(relOp *RelOp) bool {
	opType := strings.ToLower(relOp.PhysicalOp)

	// Check for join operators
	joinOperators := []string{"nested loops", "hash match", "merge join"}
	for _, joinOp := range joinOperators {
		if strings.Contains(opType, joinOp) {
			// Check if estimated rows is suspiciously high (potential Cartesian product)
			if rows, err := strconv.ParseFloat(relOp.EstimateRows, 64); err == nil && rows > 1000000 {
				return true
			}
		}
	}
	return false
}
