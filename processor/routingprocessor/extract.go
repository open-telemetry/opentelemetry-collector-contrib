// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor"

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

// extractor is responsible for extracting configured attributes from the processed data.
// Currently it can be extract the attributes from context or resource attributes.
type extractor struct {
	fromAttr string
	logger   *zap.Logger
}

// newExtractor creates new extractor which can extract attributes from logs,
// metrics and traces from from requested attribute source and from the provided
// attribute name.
func newExtractor(fromAttr string, logger *zap.Logger) extractor {
	return extractor{
		fromAttr: fromAttr,
		logger:   logger,
	}
}

// extractAttrFromResource extract string value from the requested resource attribute.
func (e extractor) extractAttrFromResource(r pcommon.Resource) string {
	firstResourceAttributes := r.Attributes()
	routingAttribute, found := firstResourceAttributes.Get(e.fromAttr)
	if !found {
		return ""
	}

	return routingAttribute.AsString()
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
