// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stdin // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/stdin"

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("stdin")

func init() {
	operator.RegisterFactory(NewFactory())
}

// Deprecated [v0.97.0] Use Factory.NewDefaultConfig instead.
func NewConfig(operatorID string) *Config {
	return NewFactory().NewDefaultConfig(operatorID).(*Config)
}

// Config is the configuration of a stdin input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
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

// CreateOperator creates a stdin input operator.
func (f *factory) CreateOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	inputOperator, err := helper.NewInput(c.InputConfig, set)
	if err != nil {
		return nil, err
	}

	return &Input{
		InputOperator: inputOperator,
		stdin:         os.Stdin,
	}, nil
}

// Input is an operator that reads input from stdin
type Input struct {
	helper.InputOperator
	wg     sync.WaitGroup
	cancel context.CancelFunc
	stdin  *os.File
}

// Start will start generating log entries.
func (g *Input) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel

	stat, err := g.stdin.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat stdin: %w", err)
	}

	if stat.Mode()&os.ModeNamedPipe == 0 {
		g.Warn("No data is being written to stdin")
		return nil
	}

	scanner := bufio.NewScanner(g.stdin)

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if ok := scanner.Scan(); !ok {
				if err := scanner.Err(); err != nil {
					g.Errorf("Scanning failed", zap.Error(err))
				}
				g.Infow("Stdin has been closed")
				return
			}

			e := entry.New()
			e.Body = scanner.Text()
			g.Write(ctx, e)
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
