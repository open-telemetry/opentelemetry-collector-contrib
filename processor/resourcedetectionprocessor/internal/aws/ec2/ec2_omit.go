// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (remove_all_resourcedetection_detectors && !enable_detector_aws_ec2) || omit_detector_aws_ec2

package ec2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"

import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"

// NewDetector is unavailable when this detector is excluded at build time.
var NewDetector internal.DetectorFactory
