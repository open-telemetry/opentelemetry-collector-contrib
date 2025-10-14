package models

import (
	"database/sql"
	"time"
)

// WaitEvent represents a wait event record from Oracle wait events queries
type WaitEvent struct {
	DatabaseName         sql.NullString
	QueryID              sql.NullString
	WaitCategory         sql.NullString
	WaitEventName        sql.NullString
	CollectionTimestamp  sql.NullTime
	WaitingTasksCount    sql.NullInt64
	TotalWaitTimeMs      sql.NullFloat64
	AvgWaitTimeMs        sql.NullFloat64
}

// GetDatabaseName returns the database name as a string, empty if null
func (we *WaitEvent) GetDatabaseName() string {
	if we.DatabaseName.Valid {
		return we.DatabaseName.String
	}
	return ""
}

// GetQueryID returns the query ID as a string, empty if null
func (we *WaitEvent) GetQueryID() string {
	if we.QueryID.Valid {
		return we.QueryID.String
	}
	return ""
}

// GetWaitCategory returns the wait category as a string, empty if null
func (we *WaitEvent) GetWaitCategory() string {
	if we.WaitCategory.Valid {
		return we.WaitCategory.String
	}
	return ""
}

// GetWaitEventName returns the wait event name as a string, empty if null
func (we *WaitEvent) GetWaitEventName() string {
	if we.WaitEventName.Valid {
		return we.WaitEventName.String
	}
	return ""
}

// GetCollectionTimestamp returns the collection timestamp, zero time if null
func (we *WaitEvent) GetCollectionTimestamp() time.Time {
	if we.CollectionTimestamp.Valid {
		return we.CollectionTimestamp.Time
	}
	return time.Time{}
}

// GetWaitingTasksCount returns the waiting tasks count as int64, 0 if null
func (we *WaitEvent) GetWaitingTasksCount() int64 {
	if we.WaitingTasksCount.Valid {
		return we.WaitingTasksCount.Int64
	}
	return 0
}

// GetTotalWaitTimeMs returns the total wait time in ms as float64, 0 if null
func (we *WaitEvent) GetTotalWaitTimeMs() float64 {
	if we.TotalWaitTimeMs.Valid {
		return we.TotalWaitTimeMs.Float64
	}
	return 0
}

// GetAvgWaitTimeMs returns the average wait time in ms as float64, 0 if null
func (we *WaitEvent) GetAvgWaitTimeMs() float64 {
	if we.AvgWaitTimeMs.Valid {
		return we.AvgWaitTimeMs.Float64
	}
	return 0
}

// HasValidQueryID checks if the wait event has a valid query ID
func (we *WaitEvent) HasValidQueryID() bool {
	return we.QueryID.Valid
}

// HasValidWaitEventName checks if the wait event has a valid wait event name
func (we *WaitEvent) HasValidWaitEventName() bool {
	return we.WaitEventName.Valid
}

// HasValidTotalWaitTime checks if the wait event has a valid total wait time
func (we *WaitEvent) HasValidTotalWaitTime() bool {
	return we.TotalWaitTimeMs.Valid
}

// HasValidWaitingTasksCount checks if the wait event has a valid waiting tasks count
func (we *WaitEvent) HasValidWaitingTasksCount() bool {
	return we.WaitingTasksCount.Valid
}

// HasValidAvgWaitTime checks if the wait event has a valid average wait time
func (we *WaitEvent) HasValidAvgWaitTime() bool {
	return we.AvgWaitTimeMs.Valid
}

// IsValidForMetrics checks if the wait event has the minimum required fields for metrics
func (we *WaitEvent) IsValidForMetrics() bool {
	return we.HasValidQueryID() && we.HasValidWaitEventName() && we.HasValidTotalWaitTime()
}