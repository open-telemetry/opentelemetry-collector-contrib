// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracebench

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Counter struct {
	Name     string
	Calls    int64
	Incoming int64
	Outgoing int64
	Elapsed  time.Duration
}

type pipelineStage struct {
	processor.Traces
	Counter
	output ptrace.Traces
}

type Pipeline struct {
	common  []*pipelineStage
	sampled []*pipelineStage
	stats   connector.Traces
	Stats   Counter
	Output  int64
	Bytes   int64
}

func telemetry() component.TelemetrySettings {
	return component.TelemetrySettings{
		Logger: zap.NewNop(), MeterProvider: noop.NewMeterProvider(),
		TracerProvider: trace.NewNoopTracerProvider(),
	}
}

func factory(kind string) (processor.Factory, error) {
	switch kind {
	case "transform":
		return transformprocessor.NewFactory(), nil
	case "attributes":
		return attributesprocessor.NewFactory(), nil
	case "redaction":
		return redactionprocessor.NewFactory(), nil
	case "filter":
		return filterprocessor.NewFactory(), nil
	case "probabilistic_sampler":
		return probabilisticsamplerprocessor.NewFactory(), nil
	default:
		return nil, fmt.Errorf("unsupported processor type %q", kind)
	}
}

func configure(config component.Config, values map[string]any) error {
	if err := confmap.NewFromStringMap(values).Unmarshal(config); err != nil {
		return err
	}
	if validator, ok := config.(interface{ Validate() error }); ok {
		return validator.Validate()
	}
	return nil
}

func newStage(ctx context.Context, spec Stage) (*pipelineStage, error) {
	f, err := factory(spec.Type)
	if err != nil {
		return nil, err
	}
	values := spec.Config
	if spec.File != "" {
		path, err := filepath.Abs(spec.File)
		if err != nil {
			return nil, err
		}
		resolver, err := confmap.NewResolver(confmap.ResolverSettings{
			URIs: []string{"file:" + path},
			ProviderFactories: []confmap.ProviderFactory{
				fileprovider.NewFactory(), envprovider.NewFactory(),
			},
			DefaultScheme: "env",
		})
		if err != nil {
			return nil, err
		}
		conf, resolveErr := resolver.Resolve(ctx)
		if err := errors.Join(resolveErr, resolver.Shutdown(ctx)); err != nil {
			return nil, err
		}
		values = conf.ToStringMap()
	}
	config := f.CreateDefaultConfig()
	if err := configure(config, values); err != nil {
		return nil, fmt.Errorf("configure %s: %w", spec.Name, err)
	}
	stage := &pipelineStage{Counter: Counter{Name: spec.Name}}
	sink, err := consumer.NewTraces(func(_ context.Context, data ptrace.Traces) error {
		stage.output = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	stage.Traces, err = f.CreateTraces(ctx, processor.Settings{
		ID: component.NewIDWithName(f.Type(), spec.Name), TelemetrySettings: telemetry(),
	}, config, sink)
	if err != nil {
		return nil, err
	}
	if err := stage.Start(ctx, nil); err != nil {
		return nil, errors.Join(err, stage.Shutdown(ctx))
	}
	return stage, nil
}

func NewPipeline(ctx context.Context, spec Spec) (_ *Pipeline, err error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	p := &Pipeline{Stats: Counter{Name: "stats"}}
	defer func() {
		if err != nil {
			err = errors.Join(err, p.Close(ctx))
		}
	}()
	for _, s := range spec.Common {
		stage, err := newStage(ctx, s)
		if err != nil {
			return nil, err
		}
		p.common = append(p.common, stage)
	}
	sampler, err := newStage(ctx, Stage{
		Name: "sampler", Type: "probabilistic_sampler",
		Config: map[string]any{"sampling_percentage": spec.SamplePercent, "hash_seed": 22},
	})
	if err != nil {
		return nil, err
	}
	p.sampled = append(p.sampled, sampler)
	for _, s := range spec.Sampled {
		stage, err := newStage(ctx, s)
		if err != nil {
			return nil, err
		}
		p.sampled = append(p.sampled, stage)
	}
	if spec.Stats {
		f := datadogconnector.NewFactory()
		config := f.CreateDefaultConfig()
		if err := configure(config, spec.StatsConfig); err != nil {
			return nil, err
		}
		sink, err := consumer.NewMetrics(func(context.Context, pmetric.Metrics) error { return nil })
		if err != nil {
			return nil, err
		}
		p.stats, err = f.CreateTracesToMetrics(ctx, connector.Settings{
			ID: component.NewID(f.Type()), TelemetrySettings: telemetry(),
		}, config, sink)
		if err != nil {
			return nil, err
		}
		if err := p.stats.Start(ctx, nil); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (s *pipelineStage) consume(ctx context.Context, data ptrace.Traces) (ptrace.Traces, error) {
	s.output = ptrace.NewTraces()
	s.Calls++
	s.Incoming += int64(data.SpanCount())
	start := time.Now()
	err := s.ConsumeTraces(ctx, data)
	s.Elapsed += time.Since(start)
	s.Outgoing += int64(s.output.SpanCount())
	return s.output, err
}

func (p *Pipeline) Consume(ctx context.Context, data ptrace.Traces) error {
	var err error
	for _, stage := range p.common {
		data, err = stage.consume(ctx, data)
		if err != nil {
			return fmt.Errorf("%s: %w", stage.Name, err)
		}
	}
	if p.stats != nil {
		p.Stats.Calls++
		p.Stats.Incoming += int64(data.SpanCount())
		start := time.Now()
		err := p.stats.ConsumeTraces(ctx, data)
		p.Stats.Elapsed += time.Since(start)
		if err != nil {
			return err
		}
		p.Stats.Outgoing += int64(data.SpanCount())
	}
	for _, stage := range p.sampled {
		data, err = stage.consume(ctx, data)
		if err != nil {
			return fmt.Errorf("%s: %w", stage.Name, err)
		}
	}
	wire, err := ptraceotlp.NewExportRequestFromTraces(data).MarshalProto()
	p.Output += int64(data.SpanCount())
	p.Bytes += int64(len(wire))
	return err
}

func (p *Pipeline) Counters() []Counter {
	var counters []Counter
	for _, stage := range p.common {
		counters = append(counters, stage.Counter)
	}
	if p.stats != nil {
		counters = append(counters, p.Stats)
	}
	for _, stage := range p.sampled {
		counters = append(counters, stage.Counter)
	}
	return counters
}

func (p *Pipeline) Close(ctx context.Context) error {
	var result error
	if p.stats != nil {
		result = errors.Join(result, p.stats.Shutdown(ctx))
	}
	for _, group := range [][]*pipelineStage{p.sampled, p.common} {
		for i := len(group) - 1; i >= 0; i-- {
			result = errors.Join(result, group[i].Shutdown(ctx))
		}
	}
	return result
}
