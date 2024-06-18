// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"
import (
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type MetricsTranslator struct {
	sync.RWMutex
	buildInfo component.BuildInfo
	lastTs    map[identity.Stream]pcommon.Timestamp
}

func newMetricsTranslator() *MetricsTranslator {
	return &MetricsTranslator{
		lastTs: make(map[identity.Stream]pcommon.Timestamp),
	}
}
