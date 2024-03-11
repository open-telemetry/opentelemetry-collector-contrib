// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package generate // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/generate"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("generate_input")

func init() {
	operator.RegisterFactory(NewFactory())
}

// Deprecated [v0.97.0] Use Factory.CreateOperator instead.
func NewConfig(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType.String()),
	}
}

// Config is the configuration of a generate input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	Entry              entry.Entry `mapstructure:"entry"`
	Count              int         `mapstructure:"count"`
	Static             bool        `mapstructure:"static"`
}

// Deprecated [v0.97.0] Use Factory.CreateOperator instead.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	set := component.TelemetrySettings{}
	if logger != nil {
		set.Logger = logger.Desugar()
	}
	return NewFactory().CreateOperator(&c, set)
}

type factory struct{}

// NewFactory creates a new factory.
func NewFactory() operator.Factory {
	return &factory{}
}

// Type gets the type of the operator.
func (f *factory) Type() component.Type {
	return operatorType
}

// NewDefaultConfig creates a new default configuration.
func (f *factory) NewDefaultConfig(operatorID string) component.Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType.String()),
	}
}

// CreateOperator creates a generate input operator.
func (f *factory) CreateOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	inputOperator, err := helper.NewInput(c.InputConfig, set)
	if err != nil {
		return nil, err
	}

	c.Entry.Body = recursiveMapInterfaceToMapString(c.Entry.Body)

	return &Input{
		InputOperator: inputOperator,
		entry:         c.Entry,
		count:         c.Count,
		static:        c.Static,
	}, nil
}

// Input is an operator that generates log entries.
type Input struct {
	helper.InputOperator
	entry  entry.Entry
	count  int
	static bool
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// Start will start generating log entries.
func (g *Input) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			entry := g.entry.Copy()
			if !g.static {
				entry.Timestamp = time.Now()
			}
			g.Write(ctx, entry)

			i++
			if i == g.count {
				return
			}
		}
	}()

	return nil
}

// Stop will stop generating logs.
func (g *Input) Stop() error {
	g.cancel()
	g.wg.Wait()
	return nil
}

func recursiveMapInterfaceToMapString(m any) any {
	switch m := m.(type) {
	case map[string]any:
		newMap := make(map[string]any)
		for k, v := range m {
			newMap[k] = recursiveMapInterfaceToMapString(v)
		}
		return newMap
	case map[any]any:
		newMap := make(map[string]any)
		for k, v := range m {
			str, ok := k.(string)
			if !ok {
				str = fmt.Sprintf("%v", k)
			}
			newMap[str] = recursiveMapInterfaceToMapString(v)
		}
		return newMap
	default:
		return m
	}
}
