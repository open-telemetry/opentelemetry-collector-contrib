// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor"

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

// extractor is responsible for extracting configured attributes from the processed data.
// Currently, it can only extract the attributes from context.
type extractor struct {
	fromAttr string
	logger   *zap.Logger
}

// newExtractor creates new extractor which can extract attributes from logs,
// metrics and traces from requested attribute source and from the provided
// attribute name.
func newExtractor(fromAttr string, logger *zap.Logger) extractor {
	return extractor{
		fromAttr: fromAttr,
		logger:   logger,
	}
}

func (e extractor) extractFromContext(ctx context.Context) string {
	// right now, we only support looking up attributes from requests that have
	// gone through the gRPC server in that case, it will add the HTTP headers
	// as context metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	// we have gRPC metadata in the context but does it have our key?
	values, ok := md[strings.ToLower(e.fromAttr)]
	if !ok {
		return ""
	}

	if len(values) > 1 {
		e.logger.Debug("more than one value found for the attribute, using only the first",
			zap.Strings("values", values),
			zap.String("attribute", e.fromAttr),
		)
	}

	return values[0]
}
