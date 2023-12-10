package metrics

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Ident struct {
	ScopeIdent

	name string
	unit string
	ty   string

	monotonic   bool
	temporality pmetric.AggregationTemporality
}

type ResourceIdent struct {
	attrs [16]byte
}

type ScopeIdent struct {
	ResourceIdent

	name    string
	version string
	attrs   [16]byte
}
