// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (remove_all_resourcedetection_detectors && !enable_detector_kubeadm) || omit_detector_kubeadm

package kubeadm // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/kubeadm"

import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"

// NewDetector is unavailable when this detector is excluded at build time.
var NewDetector internal.DetectorFactory
