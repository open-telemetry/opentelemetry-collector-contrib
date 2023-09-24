package metrics

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	jsonlib "go.starlark.net/lib/json"
	"go.starlark.net/starlark"
	"go.uber.org/zap"
)

func NewProcessor(ctx context.Context, logger *zap.Logger,
	code string, consumer consumer.Metrics) *Processor {
	return &Processor{
		logger: logger,
		code:   code,
		next:   consumer,
		thread: &starlark.Thread{
			Name: "metric/processor",
			Print: func(thread *starlark.Thread, msg string) {
				logger.Debug(msg, zap.String("thread", thread.Name), zap.String("source", "starlark/code"))
			},
		},
	}
}

type Processor struct {
	pmetric.JSONMarshaler
	pmetric.JSONUnmarshaler
	logger      *zap.Logger
	code        string
	thread      *starlark.Thread
	transformFn starlark.Value
	next        consumer.Metrics
}

func (p *Processor) Start(context.Context, component.Host) error {

	global := starlark.StringDict{
		"json": jsonlib.Module,
	}

	globals, err := starlark.ExecFile(p.thread, "", p.code, global)
	if err != nil {
		return err
	}

	// Retrieve a module global.
	var ok bool
	if p.transformFn, ok = globals["transform"]; !ok {
		return errors.New("starlark: no 'transform' function defined in script")
	}
	return nil
}

func (p *Processor) Shutdown(context.Context) error { return nil }

func (p *Processor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	b, err := p.MarshalMetrics(md)
	if err != nil {
		return err
	}

	// Call the function.
	result, err := starlark.Call(p.thread, p.transformFn, starlark.Tuple{starlark.String(string(b))}, nil)
	if err != nil {
		return fmt.Errorf("error calling transform function: %w", err)
	}

	if result.String() == "None" {
		p.logger.Error("transform function returned an empty value, passing record with no changes", zap.String("result", result.String()))
		return p.next.ConsumeMetrics(ctx, md)
	}

	if md, err = p.UnmarshalMetrics([]byte(result.String())); err != nil {
		return fmt.Errorf("error unmarshalling logs data from starlark: %w", err)
	}

	// if there are no metrics, return
	if md.ResourceMetrics().Len() == 0 {
		return nil
	}

	return p.next.ConsumeMetrics(ctx, md)
}

func (p *Processor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}
