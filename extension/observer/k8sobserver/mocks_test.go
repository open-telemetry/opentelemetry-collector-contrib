// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

type endpointSink struct {
	sync.Mutex
	added   []observer.Endpoint
	removed []observer.Endpoint
	changed []observer.Endpoint
}

func (e *endpointSink) ID() observer.NotifyID {
	return "endpointSink"
}

func (e *endpointSink) OnAdd(added []observer.Endpoint) {
	e.Lock()
	defer e.Unlock()
	e.added = append(e.added, added...)
}

func (e *endpointSink) OnRemove(removed []observer.Endpoint) {
	e.Lock()
	defer e.Unlock()
	e.removed = append(e.removed, removed...)
}

func (e *endpointSink) OnChange(changed []observer.Endpoint) {
	e.Lock()
	defer e.Unlock()
	e.changed = e.removed
	e.changed = append(e.changed, changed...)
}

var _ observer.Notify = (*endpointSink)(nil)

func requireSink(t *testing.T, sink *endpointSink, f func() bool) {
	require.Eventually(t, func() bool {
		sink.Lock()
		defer sink.Unlock()
		return f()
	}, 2*time.Second, 100*time.Millisecond)
}
