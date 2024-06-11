// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type Ident = identity.Metric

type Metric struct {
	res   pcommon.Resource
	scope pcommon.InstrumentationScope
	pmetric.Metric
}

func (m *Metric) Ident() Ident {
	return identity.OfResourceMetric(m.res, m.scope, m.Metric)
}

func From(res pcommon.Resource, scope pcommon.InstrumentationScope, metric pmetric.Metric) Metric {
	return Metric{res: res, scope: scope, Metric: metric}
}
