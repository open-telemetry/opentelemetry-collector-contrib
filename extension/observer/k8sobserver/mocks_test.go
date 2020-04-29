// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sobserver

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

type endpointSink struct {
	sync.Mutex
	added   []observer.Endpoint
	removed []observer.Endpoint
	changed []observer.Endpoint
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
	e.changed = append(e.removed, changed...)
}

var _ observer.Notify = (*endpointSink)(nil)

func assertSink(t *testing.T, sink *endpointSink, f func() bool) {
	assert.Eventually(t, func() bool {
		sink.Lock()
		defer sink.Unlock()
		return f()
	}, 1*time.Second, 100*time.Millisecond)
}
