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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	framework "k8s.io/client-go/tools/cache/testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestNewExtension(t *testing.T) {
	listWatch := framework.NewFakeControllerSource()
	factory := &Factory{}
	ext, err := newObserver(zap.NewNop(), factory.CreateDefaultConfig().(*Config), listWatch)
	require.NoError(t, err)
	require.NotNil(t, ext)
}

func TestExtensionObserve(t *testing.T) {
	listWatch := framework.NewFakeControllerSource()
	factory := &Factory{}
	ext, err := newObserver(zap.NewNop(), factory.CreateDefaultConfig().(*Config), listWatch)
	require.NoError(t, err)
	require.NotNil(t, ext)
	obs := ext.(*k8sObserver)

	listWatch.Add(pod1V1)

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	sink := &endpointSink{}
	obs.ListAndWatch(sink)

	assertSink(t, sink, func() bool {
		return len(sink.added) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/pod1-UID",
		Target: "1.2.3.4",
		Details: observer.Pod{
			Name: "pod1",
			Labels: map[string]string{
				"env": "prod",
			},
		},
	}, sink.added[0])

	listWatch.Delete(pod1V2)

	assertSink(t, sink, func() bool {
		return len(sink.removed) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/pod1-UID",
		Target: "1.2.3.4",
		Details: observer.Pod{
			Name: "pod1",
			Labels: map[string]string{
				"env":         "prod",
				"pod-version": "2",
			},
		},
	}, sink.removed[0])

	require.NoError(t, ext.Shutdown(context.Background()))
}
