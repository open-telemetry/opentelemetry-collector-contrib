// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/xray"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/component"
)

// mockClient V2용 재정의 (이미 sender_test.go에서 정의했다면 제거 가능)
type mockClientRegistry struct {
	mock.Mock
	count *atomic.Int64
}

func (m *mockClientRegistry) PutTraceSegments(ctx context.Context, input *xray.PutTraceSegmentsInput, opts ...func(*xray.Options)) (*xray.PutTraceSegmentsOutput, error) {
	args := m.Called(input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*xray.PutTraceSegmentsOutput), args.Error(1)
}

func (m *mockClientRegistry) PutTelemetryRecords(ctx context.Context, input *xray.PutTelemetryRecordsInput, opts ...func(*xray.Options)) (*xray.PutTelemetryRecordsOutput, error) {
	args := m.Called(input)
	if m.count != nil {
		m.count.Add(1)
	}
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*xray.PutTelemetryRecordsOutput), args.Error(1)
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	newID := component.MustNewID("new")
	contribID := component.MustNewID("contrib")
	notCreatedID := component.MustNewID("not_created")
	
	// 테스트를 위한 모의 클라이언트 생성
	mockXRayClient := &mockClientRegistry{}
	
	original := r.Register(
		newID,
		Config{
			IncludeMetadata: false,
			Contributors:    []component.ID{contribID},
		},
		mockXRayClient,
	)
	
	withSameID := r.Register(
		newID,
		Config{
			IncludeMetadata: true,
			Contributors:    []component.ID{notCreatedID},
		},
		mockXRayClient,
		WithResourceARN("arn"),
	)
	
	// still the same recorder
	assert.Same(t, original, withSameID)
	
	// contributors have access to same recorder
	contrib := r.Load(contribID)
	assert.NotNil(t, contrib)
	assert.Same(t, original, contrib)
	
	// second attempt with same ID did not give contributors access
	assert.Nil(t, r.Load(notCreatedID))
	
	nop := r.LoadOrNop(notCreatedID)
	assert.NotNil(t, nop)
	assert.Equal(t, NewNopSender(), nop)
}

func TestGlobalRegistry(t *testing.T) {
	assert.Same(t, globalRegistry, GlobalRegistry())
}