package decisioncache

import "go.opentelemetry.io/collector/pdata/pcommon"

// DecisionCache holds trace IDs that had sampling decisions made on them.
// It does not specify the type of sampling decision that was made, only that
// a decision was made for an ID. You need separate DecisionCaches for caching
// sampled and not sampled trace IDs.
type DecisionCache interface {
	// Get indicates that a decision was made for the given ID
	Get(id pcommon.TraceID) bool
	// Put adds a decision to the DecisionCache
	Put(id pcommon.TraceID) error
}
