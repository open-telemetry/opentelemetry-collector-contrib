// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

const (
	DbAttributePrefix           = "postgresql."
	QueryidColumnName           = "queryid"
	TotalExecTimeColumnName     = "total_exec_time"
	TotalPlanTimeColumnName     = "total_plan_time"
	CallsColumnName             = "calls"
	RowsColumnName              = "rows"
	SharedBlksDirtiedColumnName = "shared_blks_dirtied"
	SharedBlksHitColumnName     = "shared_blks_hit"
	SharedBlksReadColumnName    = "shared_blks_read"
	SharedBlksWrittenColumnName = "shared_blks_written"
	TempBlksReadColumnName      = "temp_blks_read"
	TempBlksWrittenColumnName   = "temp_blks_written"
)
