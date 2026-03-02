// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/cache"

import "go.opentelemetry.io/collector/pdata/pcommon"

type nopDecisionCache struct{}

var _ Cache = (*nopDecisionCache)(nil)

func NewNopDecisionCache() Cache {
	return &nopDecisionCache{}
}

func (*nopDecisionCache) Get(pcommon.TraceID) (DecisionMetadata, bool) {
	return DecisionMetadata{}, false
}

func (*nopDecisionCache) Put(_ pcommon.TraceID, _ DecisionMetadata) {
}
