package models

import (
	"database/sql"
	"strconv"
)

// IndividualQuery represents an individual query record from Oracle V$SQL view with user information
type IndividualQuery struct {
	QueryID       sql.NullString
	UserID        sql.NullInt64
	Username      sql.NullString
	QueryText     sql.NullString
	CPUTimeMs     sql.NullFloat64
	ElapsedTimeMs sql.NullFloat64
	Hostname      sql.NullString
	DatabaseName  sql.NullString
}

// GetQueryID returns the query ID as a string, empty if null
func (iq *IndividualQuery) GetQueryID() string {
	if iq.QueryID.Valid {
		return iq.QueryID.String
	}
	return ""
}

// GetUserID returns the user ID as a string, empty if null
func (iq *IndividualQuery) GetUserID() string {
	if iq.UserID.Valid {
		return strconv.FormatInt(iq.UserID.Int64, 10)
	}
	return ""
}

// GetUsername returns the username as a string, empty if null
func (iq *IndividualQuery) GetUsername() string {
	if iq.Username.Valid {
		return iq.Username.String
	}
	return ""
}

// GetQueryText returns the query text as a string, empty if null
func (iq *IndividualQuery) GetQueryText() string {
	if iq.QueryText.Valid {
		return iq.QueryText.String
	}
	return ""
}

// GetHostname returns the hostname as a string, empty if null
func (iq *IndividualQuery) GetHostname() string {
	if iq.Hostname.Valid {
		return iq.Hostname.String
	}
	return ""
}

// GetDatabaseName returns the database name as a string, empty if null
func (iq *IndividualQuery) GetDatabaseName() string {
	if iq.DatabaseName.Valid {
		return iq.DatabaseName.String
	}
	return ""
}

// HasValidQueryID checks if the query has a valid query ID
func (iq *IndividualQuery) HasValidQueryID() bool {
	return iq.QueryID.Valid
}

// HasValidElapsedTime checks if the query has a valid elapsed time
func (iq *IndividualQuery) HasValidElapsedTime() bool {
	return iq.ElapsedTimeMs.Valid
}

// IsValidForMetrics checks if the individual query has the minimum required fields for metrics
func (iq *IndividualQuery) IsValidForMetrics() bool {
	return iq.HasValidQueryID() && iq.HasValidElapsedTime()
}
