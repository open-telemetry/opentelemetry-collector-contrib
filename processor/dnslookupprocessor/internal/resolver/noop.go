// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/resolver"
import "context"

// TODO: Remove this file when the noop resolver is no longer needed.
type NoOpResolver struct{}

func NewNoOpResolver() *NoOpResolver {
	return &NoOpResolver{}
}

func (r *NoOpResolver) Resolve(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (r *NoOpResolver) Reverse(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
