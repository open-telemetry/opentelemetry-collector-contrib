// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/client"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

func TestExtractorForTraces_FromContext(t *testing.T) {
	testcases := []struct {
		name          string
		ctxFunc       func() context.Context
		fromAttr      string
		expectedValue string
	}{
		{
			name: "value from existing GRPC attribute",
			ctxFunc: func() context.Context {
				return metadata.NewIncomingContext(context.Background(),
					metadata.Pairs("X-Tenant", "acme"),
				)
			},
			fromAttr:      "X-Tenant",
			expectedValue: "acme",
		},
		{
			name:          "no values from empty context",
			ctxFunc:       context.Background,
			fromAttr:      "X-Tenant",
			expectedValue: "",
		},
		{
			name: "no values from existing GRPC attribute",
			ctxFunc: func() context.Context {
				return metadata.NewIncomingContext(context.Background(),
					metadata.Pairs("X-Tenant", ""),
				)
			},
			fromAttr:      "X-Tenant",
			expectedValue: "",
		},
		{
			name: "multiple values from existing GRPC attribute returns the first one",
			ctxFunc: func() context.Context {
				return metadata.NewIncomingContext(context.Background(),
					metadata.Pairs("X-Tenant", "globex", "X-Tenant", "acme"),
				)
			},
			fromAttr:      "X-Tenant",
			expectedValue: "globex",
		},
		{
			name: "value from existing HTTP attribute",
			ctxFunc: func() context.Context {
				return client.NewContext(context.Background(),
					client.Info{Metadata: client.NewMetadata(map[string][]string{
						"X-Tenant": {"acme"},
					})})
			},
			fromAttr:      "X-Tenant",
			expectedValue: "acme",
		},
		{
			name:          "no values from empty context",
			ctxFunc:       context.Background,
			fromAttr:      "X-Tenant",
			expectedValue: "",
		},
		{
			name: "no values from existing HTTP attribute",
			ctxFunc: func() context.Context {
				return client.NewContext(context.Background(),
					client.Info{Metadata: client.NewMetadata(map[string][]string{
						"X-Tenant": {""},
					})})
			},
			fromAttr:      "X-Tenant",
			expectedValue: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			e := newExtractor(tc.fromAttr, zap.NewNop())

			assert.Equal(t,
				tc.expectedValue,
				e.extractFromContext(tc.ctxFunc()),
			)
		})
	}
}
