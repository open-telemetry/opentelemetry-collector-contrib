// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type MetricsTranslator struct {
	sync.RWMutex
	buildInfo  component.BuildInfo
	lastTs     map[identity.Stream]pcommon.Timestamp
	stringPool *StringPool
}

func NewMetricsTranslator(buildInfo component.BuildInfo) *MetricsTranslator {
	return &MetricsTranslator{
		buildInfo:  buildInfo,
		lastTs:     make(map[identity.Stream]pcommon.Timestamp),
		stringPool: newStringPool(),
	}
}

func (mt *MetricsTranslator) streamHasTimestamp(stream identity.Stream) (pcommon.Timestamp, bool) {
	mt.RLock()
	defer mt.RUnlock()
	ts, ok := mt.lastTs[stream]
	return ts, ok
}

func (mt *MetricsTranslator) updateLastTsForStream(stream identity.Stream, ts pcommon.Timestamp) {
	mt.Lock()
	defer mt.Unlock()
	mt.lastTs[stream] = ts
}
