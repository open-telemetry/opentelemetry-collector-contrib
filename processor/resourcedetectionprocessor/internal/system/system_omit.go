// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build omit_detector_system

package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"

import (
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func NewDetector(p processor.Settings, dcfg internal.DetectorConfig, _ bool) (internal.Detector, error) {
	return nil, nil
}

// hostnameSourcesMap lists the hostname_sources values accepted by Config,
var hostnameSourcesMap = map[string]struct{}{
	"os":     {},
	"dns":    {},
	"cname":  {},
	"lookup": {},
}
