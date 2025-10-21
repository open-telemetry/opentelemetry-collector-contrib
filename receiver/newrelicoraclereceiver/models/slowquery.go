package models

import "database/sql"

// SlowQuery represents a slow query record from Oracle V$SQL view
type SlowQuery struct {
	DatabaseName          sql.NullString
	QueryID               sql.NullString
	SchemaName            sql.NullString
	UserName              sql.NullString // NEW: The user who parsed the statement
	LastLoadTime          sql.NullString // NEW: Time the cursor was last loaded
	SharableMemoryBytes   sql.NullInt64  // NEW: Total memory used in the Shared Pool
	PersistentMemoryBytes sql.NullInt64  // NEW: Persistent memory used by the cursor
	RuntimeMemoryBytes    sql.NullInt64  // NEW: Runtime memory used by the cursor
	StatementType         sql.NullString
	ExecutionCount        sql.NullInt64
	QueryText             sql.NullString
	AvgCPUTimeMs          sql.NullFloat64
	AvgDiskReads          sql.NullFloat64
	AvgDiskWrites         sql.NullFloat64
	AvgElapsedTimeMs      sql.NullFloat64
	HasFullTableScan      sql.NullString
}

// GetDatabaseName returns the database name as a string, empty if null
func (sq *SlowQuery) GetDatabaseName() string {
	if sq.DatabaseName.Valid {
		return sq.DatabaseName.String
	}
	return ""
}

// GetQueryID returns the query ID as a string, empty if null
func (sq *SlowQuery) GetQueryID() string {
	if sq.QueryID.Valid {
		return sq.QueryID.String
	}
	return ""
}

// GetSchemaName returns the schema name as a string, empty if null
func (sq *SlowQuery) GetSchemaName() string {
	if sq.SchemaName.Valid {
		return sq.SchemaName.String
	}
	return ""
}

// GetStatementType returns the statement type as a string, empty if null
func (sq *SlowQuery) GetStatementType() string {
	if sq.StatementType.Valid {
		return sq.StatementType.String
	}
	return ""
}

// GetQueryText returns the query text as a string, empty if null
func (sq *SlowQuery) GetQueryText() string {
	if sq.QueryText.Valid {
		return sq.QueryText.String
	}
	return ""
}

// GetUserName returns the username as a string, empty if null
func (sq *SlowQuery) GetUserName() string {
	if sq.UserName.Valid {
		return sq.UserName.String
	}
	return ""
}

// GetLastLoadTime returns the last load time as a string, empty if null
func (sq *SlowQuery) GetLastLoadTime() string {
	if sq.LastLoadTime.Valid {
		return sq.LastLoadTime.String
	}
	return ""
}

// GetSharableMemoryBytes returns the sharable memory bytes as int64, 0 if null
func (sq *SlowQuery) GetSharableMemoryBytes() int64 {
	if sq.SharableMemoryBytes.Valid {
		return sq.SharableMemoryBytes.Int64
	}
	return 0
}

// GetPersistentMemoryBytes returns the persistent memory bytes as int64, 0 if null
func (sq *SlowQuery) GetPersistentMemoryBytes() int64 {
	if sq.PersistentMemoryBytes.Valid {
		return sq.PersistentMemoryBytes.Int64
	}
	return 0
}

// GetRuntimeMemoryBytes returns the runtime memory bytes as int64, 0 if null
func (sq *SlowQuery) GetRuntimeMemoryBytes() int64 {
	if sq.RuntimeMemoryBytes.Valid {
		return sq.RuntimeMemoryBytes.Int64
	}
	return 0
}

// GetHasFullTableScan returns the full table scan flag as a string, empty if null
func (sq *SlowQuery) GetHasFullTableScan() string {
	if sq.HasFullTableScan.Valid {
		return sq.HasFullTableScan.String
	}
	return ""
}

// HasValidQueryID checks if the query has a valid query ID
func (sq *SlowQuery) HasValidQueryID() bool {
	return sq.QueryID.Valid
}

// HasValidElapsedTime checks if the query has a valid elapsed time
func (sq *SlowQuery) HasValidElapsedTime() bool {
	return sq.AvgElapsedTimeMs.Valid
}

// IsValidForMetrics checks if the slow query has the minimum required fields for metrics
func (sq *SlowQuery) IsValidForMetrics() bool {
	return sq.HasValidQueryID() && sq.HasValidElapsedTime()
}
