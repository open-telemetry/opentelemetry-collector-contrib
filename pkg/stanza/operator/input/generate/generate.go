// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package generate // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/generate"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func init() {
	operator.Register("generate_input", func() operator.Builder { return NewConfig("") })
}

// NewConfig creates a new generate input config with default values
func NewConfig(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, "generate_input"),
	}
}

// Config is the configuration of a generate input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	Entry              entry.Entry `mapstructure:"entry"`
	Count              int         `mapstructure:"count"`
	Static             bool        `mapstructure:"static"`
}

// Build will build a generate input operator.
func (c *Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
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
