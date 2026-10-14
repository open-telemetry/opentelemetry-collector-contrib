// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !remove_all_resourcedetection_detectors || enable_resourcedetection_k8s_api_detector

package resourcedetectionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8sapi"
)

func init() {
	registerDetector(k8sapi.TypeStr, k8sapi.NewDetector)
	registerDetector(k8sapi.TypeStrAlias, k8sapi.NewDeprecatedDetector)
}
