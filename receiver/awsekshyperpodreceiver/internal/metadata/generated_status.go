// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsekshyperpodreceiver/internal/metadata"

import (
	"go.opentelemetry.io/collector/component"
)

var (
	Type      = component.MustNewType("awsekshyperpodreceiver")
	ScopeName = "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsekshyperpodreceiver"
)

const (
	MetricsStability = component.StabilityLevelAlpha
)
