// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

const (
	callsColumnName = "calls"

	dbAttributePrefix           = "postgresql."
	queryidColumnName           = "queryid"
	rowsColumnName              = "rows"
	sharedBlksDirtiedColumnName = "shared_blks_dirtied"
	sharedBlksHitColumnName     = "shared_blks_hit"
	sharedBlksReadColumnName    = "shared_blks_read"
	sharedBlksWrittenColumnName = "shared_blks_written"
	tempBlksReadColumnName      = "temp_blks_read"
	tempBlksWrittenColumnName   = "temp_blks_written"
	totalExecTimeColumnName     = "total_exec_time"
	totalPlanTimeColumnName     = "total_plan_time"
)

const (
	querySampleColumnApplicationName      = "application_name"
	querySampleColumnClientAddr           = "client_addr"
	querySampleColumnClientHostname       = "client_hostname"
	querySampleColumnClientPort           = "client_port"
	querySampleColumnDatname              = "datname"
	querySampleColumnDurationMilliseconds = "duration_ms"
	querySampleColumnPID                  = "pid"
	querySampleColumnQuery                = "query"
	querySampleColumnQueryID              = "query_id"
	querySampleColumnQueryStart           = "query_start"
	querySampleColumnQueryStartTimestamp  = "_query_start_timestamp"
	querySampleColumnState                = "state"
	querySampleColumnUsename              = "usename"
	querySampleColumnWaitEvent            = "wait_event"
	querySampleColumnWaitEventType        = "wait_event_type"
)

const (
	insufficientPrivilegeQuerySampleText = "<insufficient privilege>"
	traceparentCarrierKey                = "traceparent"
)

const (
	postgresqlTotalExecTimeAttributeName = dbAttributePrefix + totalExecTimeColumnName
)
