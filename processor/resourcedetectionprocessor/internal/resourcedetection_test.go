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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
)

type MockDetector struct {
	mock.Mock
}

func (p *MockDetector) Detect(ctx context.Context) (pdata.Resource, error) {
	args := p.Called()
	return args.Get(0).(pdata.Resource), args.Error(1)
}

func TestDetect(t *testing.T) {
	md1 := &MockDetector{}
	md1.On("Detect").Return(NewResource(map[string]string{"a": "1", "b": "2"}), nil)

	md2 := &MockDetector{}
	md2.On("Detect").Return(NewResource(map[string]string{"a": "11", "c": "3"}), nil)

	got, err := Detect(context.Background(), md1, md2)
	require.NoError(t, err)

	want := NewResource(map[string]string{"a": "1", "b": "2", "c": "3"})

	got.Attributes().Sort()
	want.Attributes().Sort()
	assert.Equal(t, want, got)
}

func TestDetectResource_Error(t *testing.T) {
	md1 := &MockDetector{}
	md1.On("Detect").Return(NewResource(map[string]string{"a": "1", "b": "2"}), nil)

	md2 := &MockDetector{}
	md2.On("Detect").Return(pdata.NewResource(), errors.New("err1"))

	_, err := Detect(context.Background(), md1, md2)
	require.EqualError(t, err, "err1")
}
