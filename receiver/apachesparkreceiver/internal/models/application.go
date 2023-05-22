// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/models"

// Application represents the json returned by the api/v1/applications endpoint for each application

type Applications []Application

type Application struct {
	ApplicationID string `json:"id"`
	Name          string `json:"name"`
}
