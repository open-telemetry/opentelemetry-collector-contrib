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

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var _ consumer.Logs = (*enhancingConsumer)(nil)
var _ consumer.Metrics = (*enhancingConsumer)(nil)
var _ consumer.Traces = (*enhancingConsumer)(nil)

// enhancingConsumer adds additional resource attributes from the given endpoint environment before passing the
// telemetry to its next consumers. The added attributes vary based on the type of the endpoint.
type enhancingConsumer struct {
	logs    consumer.Logs
	metrics consumer.Metrics
	traces  consumer.Traces
	attrs   map[string]string
}

func newEnhancingConsumer(
	resources resourceAttributes,
	receiverAttributes map[string]string,
	env observer.EndpointEnv,
	endpoint observer.Endpoint,
	nextLogs consumer.Logs,
	nextMetrics consumer.Metrics,
	nextTraces consumer.Traces,
) (*enhancingConsumer, error) {
	attrs := map[string]string{}

	for _, resource := range []map[string]string{resources[endpoint.Details.Type()], receiverAttributes} {
		// Precompute values that will be inserted for each resource object passed through.
		for attr, expr := range resource {
			// If the attribute value is empty this signals to delete existing
			if expr == "" {
				delete(attrs, attr)
				continue
			}

			res, err := evalBackticksInConfigValue(expr, env)
			if err != nil {
				return nil, fmt.Errorf("failed processing resource attribute %q for endpoint %v: %w", attr, endpoint.ID, err)
			}

			val := fmt.Sprint(res)
			if val != "" {
				attrs[attr] = val
			}
		}
	}

	ec := &enhancingConsumer{attrs: attrs}
	if nextLogs != nil {
		ec.logs = nextLogs
	}
	if nextMetrics != nil {
		ec.metrics = nextMetrics
	}
	if nextTraces != nil {
		ec.traces = nextTraces
	}
	return ec, nil
}

func (*enhancingConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (ec *enhancingConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if ec.logs == nil {
		return fmt.Errorf("no log consumer available")
	}
	rl := ld.ResourceLogs()
	for i := 0; i < rl.Len(); i++ {
		ec.putAttrs(rl.At(i).Resource().Attributes())
	}

	return ec.logs.ConsumeLogs(ctx, ld)
}

func (ec *enhancingConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if ec.metrics == nil {
		return fmt.Errorf("no metric consumer available")
	}
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		ec.putAttrs(rm.At(i).Resource().Attributes())
	}

	return ec.metrics.ConsumeMetrics(ctx, md)
}

func (ec *enhancingConsumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if ec.traces == nil {
		return fmt.Errorf("no trace consumer available")
	}
	rs := td.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		ec.putAttrs(rs.At(i).Resource().Attributes())
	}

	return ec.traces.ConsumeTraces(ctx, td)
}

func (ec *enhancingConsumer) putAttrs(attrs pcommon.Map) {
	for attr, val := range ec.attrs {
		if _, found := attrs.Get(attr); !found {
			attrs.PutStr(attr, val)
		}
	}
}
