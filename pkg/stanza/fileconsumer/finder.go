// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"

// Deprecated: [v0.83.0] Use matcher.Criteria instead.
type MatchingCriteria = matcher.Criteria

// Deprecated: [v0.83.0] Use matcher.OrderingCriteria instead.
type OrderingCriteria = matcher.OrderingCriteria

// Deprecated: [v0.83.0] Use matcher.Sort instead.
type NumericSortRule = matcher.Sort

// Deprecated: [v0.83.0] Use matcher.Sort instead.
type AlphabeticalSortRule = matcher.Sort

// Deprecated: [v0.83.0] Use matcher.Sort instead.
type TimestampSortRule = matcher.Sort
