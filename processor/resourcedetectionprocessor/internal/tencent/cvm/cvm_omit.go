// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build omit_detector_tencent_cvm

package cvm // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/tencent/cvm"

import (
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func NewDetector(set processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	return nil, nil
}
