// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

const (
	dbAttributePrefix           = "postgresql."
	queryidColumnName           = "queryid"
	totalExecTimeColumnName     = "total_exec_time"
	totalPlanTimeColumnName     = "total_plan_time"
	callsColumnName             = "calls"
	rowsColumnName              = "rows"
	sharedBlksDirtiedColumnName = "shared_blks_dirtied"
	sharedBlksHitColumnName     = "shared_blks_hit"
	sharedBlksReadColumnName    = "shared_blks_read"
	sharedBlksWrittenColumnName = "shared_blks_written"
	tempBlksReadColumnName      = "temp_blks_read"
	tempBlksWrittenColumnName   = "temp_blks_written"
)

const (
	QueryTextAttributeName = "db.query.text"
	DatabaseAttributeName  = "db.namespace"
)
