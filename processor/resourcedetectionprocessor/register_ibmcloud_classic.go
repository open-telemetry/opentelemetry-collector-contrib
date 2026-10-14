// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !remove_all_resourcedetection_detectors || enable_resourcedetection_ibmcloud_classic_detector

package resourcedetectionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"

import (
	ibmcloudclassic "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/classic"
)

func init() {
	registerDetector(ibmcloudclassic.TypeStr, ibmcloudclassic.NewDetector)
}
