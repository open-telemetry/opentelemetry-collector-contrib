package decisioncache

import "go.opentelemetry.io/collector/pdata/pcommon"

type nopDecisionCache struct{}

var _ DecisionCache = (*nopDecisionCache)(nil)

func NewNopDecisionCache() DecisionCache { return &nopDecisionCache{} }

func (n *nopDecisionCache) Get(_ pcommon.TraceID) bool {
	return false
}

func (n *nopDecisionCache) Put(_ pcommon.TraceID) error {
	return nil
}
