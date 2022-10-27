// Copyright The OpenTelemetry Authors
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

package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

type MockDetector struct {
	mock.Mock
}

func (p *MockDetector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	args := p.Called()
	return args.Get(0).(pcommon.Resource), "", args.Error(1)
}

type mockDetectorConfig struct{}

func (d *mockDetectorConfig) GetConfigFromType(detectorType DetectorType) DetectorConfig {
	return nil
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name              string
		detectedResources []pcommon.Resource
		expectedResource  pcommon.Resource
		attributes        []string
	}{
		{
			name: "Detect three resources",
			detectedResources: []pcommon.Resource{
				NewResource(map[string]interface{}{"a": "1", "b": "2"}),
				NewResource(map[string]interface{}{"a": "11", "c": "3"}),
				NewResource(map[string]interface{}{"a": "12", "c": "3"}),
			},
			expectedResource: NewResource(map[string]interface{}{"a": "1", "b": "2", "c": "3"}),
			attributes:       nil,
		}, {
			name: "Detect empty resources",
			detectedResources: []pcommon.Resource{
				NewResource(map[string]interface{}{"a": "1", "b": "2"}),
				NewResource(map[string]interface{}{}),
				NewResource(map[string]interface{}{"a": "11"}),
			},
			expectedResource: NewResource(map[string]interface{}{"a": "1", "b": "2"}),
			attributes:       nil,
		}, {
			name: "Detect non-string resources",
			detectedResources: []pcommon.Resource{
				NewResource(map[string]interface{}{"bool": true, "int": int64(2), "double": 0.5}),
				NewResource(map[string]interface{}{"bool": false}),
				NewResource(map[string]interface{}{"a": "11"}),
			},
			expectedResource: NewResource(map[string]interface{}{"a": "11", "bool": true, "int": int64(2), "double": 0.5}),
			attributes:       nil,
		}, {
			name: "Filter to one attribute",
			detectedResources: []pcommon.Resource{
				NewResource(map[string]interface{}{"a": "1", "b": "2"}),
				NewResource(map[string]interface{}{"a": "11", "c": "3"}),
				NewResource(map[string]interface{}{"a": "12", "c": "3"}),
			},
			expectedResource: NewResource(map[string]interface{}{"a": "1"}),
			attributes:       []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDetectors := make(map[DetectorType]DetectorFactory, len(tt.detectedResources))
			mockDetectorTypes := make([]DetectorType, 0, len(tt.detectedResources))

			for i, res := range tt.detectedResources {
				md := &MockDetector{}
				md.On("Detect").Return(res, nil)

				mockDetectorType := DetectorType(fmt.Sprintf("mockdetector%v", i))
				mockDetectors[mockDetectorType] = func(component.ProcessorCreateSettings, DetectorConfig) (Detector, error) {
					return md, nil
				}
				mockDetectorTypes = append(mockDetectorTypes, mockDetectorType)
			}

			f := NewProviderFactory(mockDetectors)
			p, err := f.CreateResourceProvider(componenttest.NewNopProcessorCreateSettings(), time.Second, tt.attributes, &mockDetectorConfig{}, mockDetectorTypes...)
			require.NoError(t, err)

			got, _, err := p.Get(context.Background(), http.DefaultClient)
			require.NoError(t, err)

			tt.expectedResource.Attributes().Sort()
			got.Attributes().Sort()
			assert.Equal(t, tt.expectedResource, got)
		})
	}
}

func TestDetectResource_InvalidDetectorType(t *testing.T) {
	mockDetectorKey := DetectorType("mock")
	p := NewProviderFactory(map[DetectorType]DetectorFactory{})
	_, err := p.CreateResourceProvider(componenttest.NewNopProcessorCreateSettings(), time.Second, nil, &mockDetectorConfig{}, mockDetectorKey)
	require.EqualError(t, err, fmt.Sprintf("invalid detector key: %v", mockDetectorKey))
}

func TestDetectResource_DetectoryFactoryError(t *testing.T) {
	mockDetectorKey := DetectorType("mock")
	p := NewProviderFactory(map[DetectorType]DetectorFactory{
		mockDetectorKey: func(component.ProcessorCreateSettings, DetectorConfig) (Detector, error) {
			return nil, errors.New("creation failed")
		},
	})
	_, err := p.CreateResourceProvider(componenttest.NewNopProcessorCreateSettings(), time.Second, nil, &mockDetectorConfig{}, mockDetectorKey)
	require.EqualError(t, err, fmt.Sprintf("failed creating detector type %q: %v", mockDetectorKey, "creation failed"))
}

func TestDetectResource_Error(t *testing.T) {
	md1 := &MockDetector{}
	md1.On("Detect").Return(NewResource(map[string]interface{}{"a": "1", "b": "2"}), nil)

	md2 := &MockDetector{}
	md2.On("Detect").Return(pcommon.NewResource(), errors.New("err1"))

	p := NewResourceProvider(zap.NewNop(), time.Second, nil, md1, md2)
	_, _, err := p.Get(context.Background(), http.DefaultClient)
	require.NoError(t, err)
}

func TestMergeResource(t *testing.T) {
	for _, tt := range []struct {
		name       string
		res1       pcommon.Resource
		res2       pcommon.Resource
		overrideTo bool
		expected   pcommon.Resource
	}{
		{
			name:       "override non-empty resources",
			res1:       NewResource(map[string]interface{}{"a": "11", "b": "2"}),
			res2:       NewResource(map[string]interface{}{"a": "1", "c": "3"}),
			overrideTo: true,
			expected:   NewResource(map[string]interface{}{"a": "1", "b": "2", "c": "3"}),
		}, {
			name:       "empty resource",
			res1:       pcommon.NewResource(),
			res2:       NewResource(map[string]interface{}{"a": "1", "c": "3"}),
			overrideTo: false,
			expected:   NewResource(map[string]interface{}{"a": "1", "c": "3"}),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out := pcommon.NewResource()
			tt.res1.CopyTo(out)
			MergeResource(out, tt.res2, tt.overrideTo)
			tt.expected.Attributes().Sort()
			out.Attributes().Sort()
			assert.Equal(t, tt.expected, out)
		})
	}
}

type MockParallelDetector struct {
	mock.Mock
	ch chan struct{}
}

func NewMockParallelDetector() *MockParallelDetector {
	return &MockParallelDetector{ch: make(chan struct{})}
}

func (p *MockParallelDetector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	<-p.ch
	args := p.Called()
	return args.Get(0).(pcommon.Resource), "", args.Error(1)
}

// TestDetectResource_Parallel validates that Detect is only called once, even if there
// are multiple calls to ResourceProvider.Get
func TestDetectResource_Parallel(t *testing.T) {
	const iterations = 5

	md1 := NewMockParallelDetector()
	md1.On("Detect").Return(NewResource(map[string]interface{}{"a": "1", "b": "2"}), nil)

	md2 := NewMockParallelDetector()
	md2.On("Detect").Return(NewResource(map[string]interface{}{"a": "11", "c": "3"}), nil)

	md3 := NewMockParallelDetector()
	md3.On("Detect").Return(pcommon.NewResource(), errors.New("an error"))

	expectedResource := NewResource(map[string]interface{}{"a": "1", "b": "2", "c": "3"})
	expectedResource.Attributes().Sort()

	p := NewResourceProvider(zap.NewNop(), time.Second, nil, md1, md2, md3)

	// call p.Get multiple times
	wg := &sync.WaitGroup{}
	var m sync.Mutex
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			detected, _, err := p.Get(context.Background(), http.DefaultClient)
			require.NoError(t, err)
			m.Lock()
			detected.Attributes().Sort()
			m.Unlock()
			assert.Equal(t, expectedResource, detected)
		}()
	}

	// wait until all goroutines are blocked
	time.Sleep(5 * time.Millisecond)

	// detector.Detect should only be called once, so we only need to notify each channel once
	md1.ch <- struct{}{}
	md2.ch <- struct{}{}
	md3.ch <- struct{}{}

	// then wait until all goroutines are finished, and ensure p.Detect was only called once
	wg.Wait()
	md1.AssertNumberOfCalls(t, "Detect", 1)
	md2.AssertNumberOfCalls(t, "Detect", 1)
	md3.AssertNumberOfCalls(t, "Detect", 1)
}

func TestFilterAttributes_Match(t *testing.T) {
	m := map[string]struct{}{
		"host.name": {},
		"host.id":   {},
	}
	attr := pcommon.NewMap()
	attr.PutStr("host.name", "test")
	attr.PutStr("host.id", "test")
	attr.PutStr("drop.this", "test")

	droppedAttributes := filterAttributes(attr, m)

	_, ok := attr.Get("host.name")
	assert.True(t, ok)

	_, ok = attr.Get("host.id")
	assert.True(t, ok)

	_, ok = attr.Get("drop.this")
	assert.False(t, ok)

	assert.Contains(t, droppedAttributes, "drop.this")
}

func TestFilterAttributes_NoMatch(t *testing.T) {
	m := map[string]struct{}{
		"cloud.region": {},
	}
	attr := pcommon.NewMap()
	attr.PutStr("host.name", "test")
	attr.PutStr("host.id", "test")

	droppedAttributes := filterAttributes(attr, m)

	_, ok := attr.Get("host.name")
	assert.False(t, ok)

	_, ok = attr.Get("host.id")
	assert.False(t, ok)

	assert.EqualValues(t, droppedAttributes, []string{"host.name", "host.id"})
}

func TestFilterAttributes_NilAttributes(t *testing.T) {
	var m map[string]struct{}
	attr := pcommon.NewMap()
	attr.PutStr("host.name", "test")
	attr.PutStr("host.id", "test")

	droppedAttributes := filterAttributes(attr, m)

	_, ok := attr.Get("host.name")
	assert.True(t, ok)

	_, ok = attr.Get("host.id")
	assert.True(t, ok)

	assert.Equal(t, len(droppedAttributes), 0)
}

func TestFilterAttributes_NoAttributes(t *testing.T) {
	m := make(map[string]struct{})
	attr := pcommon.NewMap()
	attr.PutStr("host.name", "test")
	attr.PutStr("host.id", "test")

	droppedAttributes := filterAttributes(attr, m)

	_, ok := attr.Get("host.name")
	assert.True(t, ok)

	_, ok = attr.Get("host.id")
	assert.True(t, ok)

	assert.Equal(t, len(droppedAttributes), 0)
}

func TestAttributesToMap(t *testing.T) {
	m := map[string]interface{}{
		"str":    "a",
		"int":    int64(5),
		"double": 5.0,
		"bool":   true,
		"map": map[string]interface{}{
			"inner": "val",
		},
		"array": []interface{}{
			"inner",
			int64(42),
		},
	}
	attr := pcommon.NewMap()
	attr.PutStr("str", "a")
	attr.PutInt("int", 5)
	attr.PutDouble("double", 5.0)
	attr.PutBool("bool", true)
	attr.PutEmptyMap("map").PutStr("inner", "val")

	arrayAttr := attr.PutEmptySlice("array")
	arrayAttr.EnsureCapacity(2)
	arrayAttr.AppendEmpty().SetStr("inner")
	arrayAttr.AppendEmpty().SetInt(42)

	assert.Equal(t, m, AttributesToMap(attr))
}
