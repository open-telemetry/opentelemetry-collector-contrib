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
	querySampleColumnDatname              = "datname"
	querySampleColumnUsename              = "usename"
	querySampleColumnClientAddr           = "client_addr"
	querySampleColumnClientHostname       = "client_hostname"
	querySampleColumnClientPort           = "client_port"
	querySampleColumnQueryStart           = "query_start"
	querySampleColumnWaitEventType        = "wait_event_type"
	querySampleColumnWaitEvent            = "wait_event"
	querySampleColumnQueryID              = "query_id"
	querySampleColumnPID                  = "pid"
	querySampleColumnApplicationName      = "application_name"
	querySampleColumnQueryStartTimestamp  = "_query_start_timestamp"
	querySampleColumnState                = "state"
	querySampleColumnQuery                = "query"
	querySampleColumnDurationMilliseconds = "duration_ms"
)

const (
	traceparentCarrierKey                = "traceparent"
	insufficientPrivilegeQuerySampleText = "<insufficient privilege>"
)

const (
	postgresqlTotalExecTimeAttributeName = dbAttributePrefix + totalExecTimeColumnName
)
