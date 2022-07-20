// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package joinattrprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/joinattrprocessor"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type joinAttrProcessor struct {
	// JoinAttributes processor configuration
	config *Config
	// // Logger
	// logger *zap.Logger
}

// newJoinAttrProcessor creates a new instance of the joinattr processor
func newJoinAttrProcessor(ctx context.Context, config *Config) (*joinAttrProcessor, error) {
	if config == nil {
		return nil, errors.New("no configuration provided")
	}

	if len(config.JoinAttributes) == 0 {
		return nil, fmt.Errorf("%s field is required", "join_attributes")
	}

	if config.TargetAttribute == "" {
		return nil, fmt.Errorf("%s field is required", "target_attribute")
	}

	if config.Separator == "" {
		return nil, fmt.Errorf("%s field is required", "separator")
	}

	return &joinAttrProcessor{
		config: config,
	}, nil
}

// Start the joinAttrProcessor processor
func (s *joinAttrProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown the joinAttrProcessor processor
func (s *joinAttrProcessor) Shutdown(context.Context) error {
	return nil
}

// Capabilities specifies what this processor does, such as whether it mutates data
func (s *joinAttrProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// processTraces implements ProcessMetricsFunc. It processes the incoming data
// and returns the data to be sent to the next component
func (s *joinAttrProcessor) processTraces(ctx context.Context, batch ptrace.Traces) (ptrace.Traces, error) {
	for i := 0; i < batch.ResourceSpans().Len(); i++ {
		rs := batch.ResourceSpans().At(i)
		s.processResourceSpan(ctx, rs)
	}
	return batch, nil
}

// processResourceSpan processes the RS and all of its spans and then returns the last
// view metric context. The context can be used for tests
func (s *joinAttrProcessor) processResourceSpan(ctx context.Context, rs ptrace.ResourceSpans) {
	rsAttrs := rs.Resource().Attributes()

	// Attributes can be part of a resource span
	s.processAttrs(ctx, &rsAttrs)

	for j := 0; j < rs.ScopeSpans().Len(); j++ {
		ils := rs.ScopeSpans().At(j)
		for k := 0; k < ils.Spans().Len(); k++ {
			span := ils.Spans().At(k)
			spanAttrs := span.Attributes()

			// Attributes can also be part of span
			s.processAttrs(ctx, &spanAttrs)
		}
	}
}

// processAttrs sets the new attribute of a resource span or a span
func (s *joinAttrProcessor) processAttrs(_ context.Context, attributes *pcommon.Map) {
	joinedAttr := s.getJoinedAttr(attributes)
	if s.config.Override {
		attributes.UpsertString(s.config.TargetAttribute, joinedAttr)
	} else {
		attributes.InsertString(s.config.TargetAttribute, joinedAttr)
	}
}

// getJoinedAttrs performs the join operation on the selected attributes' values
func (s *joinAttrProcessor) getJoinedAttr(attributes *pcommon.Map) string {
	values := []string{}
	for _, attr := range s.config.JoinAttributes {
		val, exists := attributes.Get(attr)
		if exists {
			values = append(values, val.StringVal())
		}
	}
	return strings.Join(values, s.config.Separator)
}
