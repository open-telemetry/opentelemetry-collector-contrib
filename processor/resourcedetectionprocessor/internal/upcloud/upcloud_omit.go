// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build omit_detector_upcloud

package upcloud // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/upcloud"

import (
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func NewDetector(p processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	return nil, nil
}
