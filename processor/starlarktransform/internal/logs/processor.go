package logs

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	jsonlib "go.starlark.net/lib/json"
	"go.starlark.net/starlark"
	"go.uber.org/zap"
)

func NewProcessor(ctx context.Context, logger *zap.Logger,
	code string, consumer consumer.Logs) *Processor {
	return &Processor{
		logger: logger,
		code:   code,
		next:   consumer,
		thread: &starlark.Thread{
			Name: "log/processor",
			Print: func(thread *starlark.Thread, msg string) {
				logger.Debug(msg, zap.String("thread", thread.Name), zap.String("source", "starlark/code"))
			},
		},
	}
}

type Processor struct {
	plog.JSONMarshaler
	plog.JSONUnmarshaler
	logger      *zap.Logger
	code        string
	thread      *starlark.Thread
	transformFn starlark.Value
	next        consumer.Logs
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

func (p *Processor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	b, err := p.MarshalLogs(ld)
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
		return p.next.ConsumeLogs(ctx, ld)
	}

	if ld, err = p.UnmarshalLogs([]byte(result.String())); err != nil {
		return fmt.Errorf("error unmarshalling logs data from starlark: %w", err)
	}

	// if there are no logs, return
	if ld.LogRecordCount() == 0 {
		return nil
	}

	return p.next.ConsumeLogs(ctx, ld)
}

func (p *Processor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}
