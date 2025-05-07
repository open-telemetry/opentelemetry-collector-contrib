// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/models"

type Attempts struct {
	StartTime        string `json:"startTime"`
	EndTime          string `json:"endTime"`
	LastUpdated      string `json:"lastUpdated"`
	Duration         int64  `json:"duration"`
	SparkUser        string `json:"sparkUser"`
	Completed        bool   `json:"completed"`
	AppSparkVersion  string `json:"appSparkVersion"`
	EndTimeEpoch     int64  `json:"endTimeEpoch"`
	StartTimeEpoch   int64  `json:"startTimeEpoch"`
	LastUpdatedEpoch int64  `json:"lastUpdatedEpoch"`
}

// Applications represents the json returned by the api/v1/applications endpoint
type Application struct {
	ApplicationID string     `json:"id"`
	Name          string     `json:"name"`
	Attempts      []Attempts `json:"attempts,omitempty"`
}
