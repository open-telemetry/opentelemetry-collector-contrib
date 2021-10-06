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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type MockDetector struct {
	mock.Mock
}

func (p *MockDetector) Detect(ctx context.Context) (pdata.Resource, string, error) {
	args := p.Called()
	return args.Get(0).(pdata.Resource), "", args.Error(1)
}

type mockDetectorConfig struct{}

func (d *mockDetectorConfig) GetConfigFromType(detectorType DetectorType) DetectorConfig {
	return nil
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name              string
		detectedResources []pdata.Resource
		expectedResource  pdata.Resource
	}{
		{
			name: "Detect three resources",
			detectedResources: []pdata.Resource{
				NewResource(map[string]interface{}{"a": "1", "b": "2"}),
				NewResource(map[string]interface{}{"a": "11", "c": "3"}),
				NewResource(map[string]interface{}{"a": "12", "c": "3"}),
			},
			expectedResource: NewResource(map[string]interface{}{"a": "1", "b": "2", "c": "3"}),
		}, {
			name: "Detect empty resources",
			detectedResources: []pdata.Resource{
				NewResource(map[string]interface{}{"a": "1", "b": "2"}),
				NewResource(map[string]interface{}{}),
				NewResource(map[string]interface{}{"a": "11"}),
			},
			expectedResource: NewResource(map[string]interface{}{"a": "1", "b": "2"}),
		}, {
			name: "Detect non-string resources",
			detectedResources: []pdata.Resource{
				NewResource(map[string]interface{}{"bool": true, "int": int64(2), "double": 0.5}),
				NewResource(map[string]interface{}{"bool": false}),
				NewResource(map[string]interface{}{"a": "11"}),
			},
			expectedResource: NewResource(map[string]interface{}{"a": "11", "bool": true, "int": int64(2), "double": 0.5}),
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
			p, err := f.CreateResourceProvider(componenttest.NewNopProcessorCreateSettings(), time.Second, &mockDetectorConfig{}, mockDetectorTypes...)
			require.NoError(t, err)

			got, _, err := p.Get(context.Background())
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
	_, err := p.CreateResourceProvider(componenttest.NewNopProcessorCreateSettings(), time.Second, &mockDetectorConfig{}, mockDetectorKey)
	require.EqualError(t, err, fmt.Sprintf("invalid detector key: %v", mockDetectorKey))
}

func TestDetectResource_DetectoryFactoryError(t *testing.T) {
	mockDetectorKey := DetectorType("mock")
	p := NewProviderFactory(map[DetectorType]DetectorFactory{
		mockDetectorKey: func(component.ProcessorCreateSettings, DetectorConfig) (Detector, error) {
			return nil, errors.New("creation failed")
		},
	})
	_, err := p.CreateResourceProvider(componenttest.NewNopProcessorCreateSettings(), time.Second, &mockDetectorConfig{}, mockDetectorKey)
	require.EqualError(t, err, fmt.Sprintf("failed creating detector type %q: %v", mockDetectorKey, "creation failed"))
}

func TestDetectResource_Error(t *testing.T) {
	md1 := &MockDetector{}
	md1.On("Detect").Return(NewResource(map[string]interface{}{"a": "1", "b": "2"}), nil)

	md2 := &MockDetector{}
	md2.On("Detect").Return(pdata.NewResource(), errors.New("err1"))

	p := NewResourceProvider(zap.NewNop(), time.Second, md1, md2)
	_, _, err := p.Get(context.Background())
	require.NoError(t, err)
}

func TestMergeResource(t *testing.T) {
	for _, tt := range []struct {
		name       string
		res1       pdata.Resource
		res2       pdata.Resource
		overrideTo bool
		expected   pdata.Resource
	}{
		{
			name:       "override non-empty resources",
			res1:       NewResource(map[string]interface{}{"a": "11", "b": "2"}),
			res2:       NewResource(map[string]interface{}{"a": "1", "c": "3"}),
			overrideTo: true,
			expected:   NewResource(map[string]interface{}{"a": "1", "b": "2", "c": "3"}),
		}, {
			name:       "empty resource",
			res1:       pdata.NewResource(),
			res2:       NewResource(map[string]interface{}{"a": "1", "c": "3"}),
			overrideTo: false,
			expected:   NewResource(map[string]interface{}{"a": "1", "c": "3"}),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out := pdata.NewResource()
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

func (p *MockParallelDetector) Detect(ctx context.Context) (pdata.Resource, string, error) {
	<-p.ch
	args := p.Called()
	return args.Get(0).(pdata.Resource), "", args.Error(1)
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
	md3.On("Detect").Return(pdata.NewResource(), errors.New("an error"))

	expectedResource := NewResource(map[string]interface{}{"a": "1", "b": "2", "c": "3"})
	expectedResource.Attributes().Sort()

	p := NewResourceProvider(zap.NewNop(), time.Second, md1, md2, md3)

	// call p.Get multiple times
	wg := &sync.WaitGroup{}
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			detected, _, err := p.Get(context.Background())
			require.NoError(t, err)
			detected.Attributes().Sort()
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
	attr := pdata.NewAttributeMap()
	attr.InsertString("str", "a")
	attr.InsertInt("int", 5)
	attr.InsertDouble("double", 5.0)
	attr.InsertBool("bool", true)
	avm := pdata.NewAttributeValueMap()
	innerAttr := avm.MapVal()
	innerAttr.InsertString("inner", "val")
	attr.Insert("map", avm)

	ava := pdata.NewAttributeValueArray()
	arrayAttr := ava.ArrayVal()
	arrayAttr.EnsureCapacity(2)
	arrayAttr.AppendEmpty().SetStringVal("inner")
	arrayAttr.AppendEmpty().SetIntVal(42)
	attr.Insert("array", ava)

	assert.Equal(t, m, AttributesToMap(attr))
}

func TestGOOSToOsType(t *testing.T) {
	assert.Equal(t, "DARWIN", GOOSToOSType("darwin"))
	assert.Equal(t, "LINUX", GOOSToOSType("linux"))
	assert.Equal(t, "WINDOWS", GOOSToOSType("windows"))
	assert.Equal(t, "DRAGONFLYBSD", GOOSToOSType("dragonfly"))
}
