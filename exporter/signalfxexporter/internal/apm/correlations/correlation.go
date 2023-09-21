// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Originally copied from https://github.com/signalfx/signalfx-agent/blob/fbc24b0fdd3884bd0bbfbd69fe3c83f49d4c0b77/pkg/apm/correlations/correlation.go

package correlations // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/correlations"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/log"
)

// Type is the type of correlation
type Type string

const (
	// Service is for correlating services
	Service Type = "service"
	// Environment is for correlating environments
	Environment Type = "environment"
)

// Correlation is a struct referencing
type Correlation struct {
	// Type is the type of correlation
	Type Type
	// DimName is the dimension name
	DimName string
	// DimValue is the dimension value
	DimValue string
	// Value is the value to makeRequest with the DimName and DimValue
	Value string
}

func (c *Correlation) Logger(l log.Logger) log.Logger {
	return l.WithFields(log.Fields{
		"correlation.type":     c.Type,
		"correlation.dimName":  c.DimName,
		"correlation.dimValue": c.DimValue,
		"correlation.value":    c.Value,
	})
}
