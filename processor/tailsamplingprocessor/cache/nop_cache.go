// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/cache"

import "go.opentelemetry.io/collector/pdata/pcommon"

type nopDecisionCache[V any] struct{}

var _ Cache[any] = (*nopDecisionCache[any])(nil)

func NewNopDecisionCache[V any]() Cache[V] {
	return &nopDecisionCache[V]{}
}

func (n *nopDecisionCache[V]) Get(_ pcommon.TraceID) (V, bool) {
	var v V
	return v, false
}

func (n *nopDecisionCache[V]) Put(_ pcommon.TraceID, _ V) {
}

func (n *nopDecisionCache[V]) Delete(_ pcommon.TraceID) {}
