// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build omit_detector_oraclecloud

package oraclecloud // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/oraclecloud"

import (
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func NewDetector(p processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	return nil, nil
}
