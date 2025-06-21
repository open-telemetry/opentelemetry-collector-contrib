// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/testutil"

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockResolver implements the resolver.Resolver interface for testing
type MockResolver struct {
	mock.Mock
}

func (m *MockResolver) Resolve(ctx context.Context, hostname string) ([]string, error) {
	args := m.Called(ctx, hostname)
	return getStrings(args, 0), args.Error(1)
}

func (m *MockResolver) Reverse(ctx context.Context, ip string) ([]string, error) {
	args := m.Called(ctx, ip)
	return getStrings(args, 0), args.Error(1)
}

func (m *MockResolver) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockResolver) Close() error {
	args := m.Called()
	return args.Error(0)
}

func getStrings(args mock.Arguments, index int) []string {
	val := args.Get(index)
	if val == nil {
		return nil
	}
	return val.([]string)
}
