// Copyright The OpenTelemetry Authors// Copyright The OpenTelemetry Authors

// SPDX-License-Identifier: Apache-2.0// SPDX-License-Identifier: Apache-2.0



package modelspackage models



import (import (

	"database/sql"	"database/sql"

))



// ExecutionPlan represents a comprehensive Oracle execution plan in XML format// ExecutionPlan represents a comprehensive Oracle execution plan in XML format

type ExecutionPlan struct {type ExecutionPlan struct {

	DatabaseName     sql.NullString `json:"database_name"`	DatabaseName     sql.NullString `json:"database_name"`

	QueryID          sql.NullString `json:"query_id"`	QueryID          sql.NullString `json:"query_id"`

	PlanHashValue    sql.NullInt64  `json:"plan_hash_value"`	PlanHashValue    sql.NullInt64  `json:"plan_hash_value"`

	ExecutionPlanXML sql.NullString `json:"execution_plan_xml"`	ExecutionPlanXML sql.NullString `json:"execution_plan_xml"`

}}



// GetDatabaseName returns the database name as a string, handling NULL values// GetDatabaseName returns the database name as a string, handling NULL values

func (ep *ExecutionPlan) GetDatabaseName() string {func (ep *ExecutionPlan) GetDatabaseName() string {

	if ep.DatabaseName.Valid {	if ep.DatabaseName.Valid {

		return ep.DatabaseName.String		return ep.DatabaseName.String

	}	}

	return ""	return ""

}}



// GetQueryID returns the query ID as a string, handling NULL values// GetQueryID returns the query ID as a string, handling NULL values

func (ep *ExecutionPlan) GetQueryID() string {func (ep *ExecutionPlan) GetQueryID() string {

	if ep.QueryID.Valid {	if ep.QueryID.Valid {

		return ep.QueryID.String		return ep.QueryID.String

	}	}

	return ""	return ""

}}



// GetPlanHashValue returns the plan hash value as an int64, handling NULL values// GetPlanHashValue returns the plan hash value as an int64, handling NULL values

func (ep *ExecutionPlan) GetPlanHashValue() int64 {func (ep *ExecutionPlan) GetPlanHashValue() int64 {

	if ep.PlanHashValue.Valid {	if ep.PlanHashValue.Valid {

		return ep.PlanHashValue.Int64		return ep.PlanHashValue.Int64

	}	}

	return 0	return 0

}}



// GetExecutionPlanXML returns the comprehensive execution plan XML as a string, handling NULL values// GetExecutionPlanXML returns the comprehensive execution plan XML as a string, handling NULL values

func (ep *ExecutionPlan) GetExecutionPlanXML() string {func (ep *ExecutionPlan) GetExecutionPlanXML() string {

	if ep.ExecutionPlanXML.Valid {	if ep.ExecutionPlanXML.Valid {

		return ep.ExecutionPlanXML.String		return ep.ExecutionPlanXML.String

	}	}

	return ""	return ""

}}



// IsValidForMetrics checks if the execution plan has the minimum required fields for metrics// IsValidForMetrics checks if the execution plan has the minimum required fields for metrics

func (ep *ExecutionPlan) IsValidForMetrics() bool {func (ep *ExecutionPlan) IsValidForMetrics() bool {

	return ep.QueryID.Valid && ep.ExecutionPlanXML.Valid && len(ep.GetExecutionPlanXML()) > 0	return ep.QueryID.Valid && ep.ExecutionPlanXML.Valid && len(ep.GetExecutionPlanXML()) > 0

}}
