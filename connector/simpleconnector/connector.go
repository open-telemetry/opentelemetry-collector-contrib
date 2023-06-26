package simpleconnector

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// schema for connector
type connectorImp struct {
	config          Config
	metricsConsumer consumer.Metrics
	count           int
	logger          *zap.Logger
}

// function to create a new connector
func newConnector(logger *zap.Logger, config component.Config) (*connectorImp, error) {
	logger.Info("Building simpleconnector connector")
	cfg := config.(*Config)

	return &connectorImp{
		config: *cfg,
		logger: logger,
	}, nil
}

// Capabilities implements the consumer interface. Necessary function for connector
func (c *connectorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *connectorImp) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var errors error
	countMetrics := pmetric.NewMetrics()
	countMetrics.ResourceMetrics().EnsureCapacity(td.ResourceSpans().Len())
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		// spansCounter := newCounter[ottlspan.TransformContext](c.spansMetricDefs)
		// spanEventsCounter := newCounter[ottlspanevent.TransformContext](c.spanEventsMetricDefs)

		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)

			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)
				// fmt.Print("list of attrs: ", span.Attributes())
				attrs := span.Attributes()
				mapping := attrs.AsRaw()

				for key, element := range mapping {
					fmt.Println("At span")
					fmt.Println("Key:", key, "=>", "Element:", element)
					fmt.Println("value of count: ", c.count)

					if key == "request.n" {
						fmt.Println("has type of request.n")
					}
					c.count++
				}

				// sCtx := ottlspan.NewTransformContext(span, scopeSpan.Scope(), resourceSpan.Resource()) // create new context
				// errors = multierr.Append(errors, spansCounter.update(ctx, span.Attributes(), sCtx)) // conut any errors

				for l := 0; l < span.Events().Len(); l++ {
					fmt.Print("here")
					// event := span.Events().At(l)
					// eCtx := ottlspanevent.NewTransformContext(event, span, scopeSpan.Scope(), resourceSpan.Resource())
					// errors = multierr.Append(errors, spanEventsCounter.update(ctx, event.Attributes(), eCtx))
				}

			}
		}

		fmt.Println("* value of count: ", c.count)
		fmt.Println("==========")

		// if len(spansCounter.counts)+len(spanEventsCounter.counts) == 0 {
		// 	continue // don't add an empty resource
		// }

		// countResource := countMetrics.ResourceMetrics().AppendEmpty()
		// resourceSpan.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		// countResource.ScopeMetrics().EnsureCapacity(resourceSpan.ScopeSpans().Len())
		// countScope := countResource.ScopeMetrics().AppendEmpty()
		// countScope.Scope().SetName(scopeName)

		// spansCounter.appendMetricsTo(countScope.Metrics())
		// spanEventsCounter.appendMetricsTo(countScope.Metrics())
	}
	if errors != nil {
		return errors
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, countMetrics)
}

// Start implements the component.Component interface.
func (p *connectorImp) Start(ctx context.Context, _ component.Host) error {
	p.logger.Info("Starting spanmetrics connector")

	// p.started = true
	// go func() {
	// 	for {
	// 		select {
	// 		case <-p.done:
	// 			return
	// 		case <-p.ticker.C:
	// 			p.exportMetrics(ctx)
	// 		}
	// 	}
	// }()

	return nil
}

// Shutdown implements the component.Component interface.
func (p *connectorImp) Shutdown(context.Context) error {
	p.logger.Info("Shutting down spanmetrics connector")
	return nil
}
