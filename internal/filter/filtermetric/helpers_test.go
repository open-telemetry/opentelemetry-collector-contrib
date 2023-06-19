// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filtermetric

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
)

func createConfig(filters []string, matchType filterset.MatchType) *MatchProperties {
	return &MatchProperties{
		MatchType:   MatchType(matchType),
		MetricNames: filters,
	}
}
